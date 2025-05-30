// Copyright (c) 2023 RightScale, Inc. - see LICENSE

package tunnel

import (
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"gopkg.in/inconshreveable/log15.v2"
)

var _ = Describe("Testing token request timeout", func() {

	var server *ghttp.Server
	var listener net.Listener
	var wstunsrv *WSTunnelServer
	var wstuncli *WSTunnelClient
	var wstunURL string
	var wstunToken string

	BeforeEach(func() {
		// start ghttp to simulate target server
		wstunToken = "test567890123456-" + strconv.Itoa(rand.Int()%1000000)
		server = ghttp.NewServer()
		log15.Info("ghttp started", "url", server.URL())

		// start wstunsrv with a very short timeout
		listener, _ = net.Listen("tcp", "127.0.0.1:0")
		wstunsrv = NewWSTunnelServer([]string{
			"-httptimeout", "2", // 2 second timeout for HTTP requests
		})
		wstunsrv.Start(listener)

		// start wstuncli
		wstuncli = NewWSTunnelClient([]string{
			"-token", wstunToken,
			"-tunnel", "ws://" + listener.Addr().String(),
			"-server", server.URL(),
			"-timeout", "3",
		})
		if err := wstuncli.Start(); err != nil {
			log15.Error("Error starting client", "error", err)
			os.Exit(1)
		}
		wstunURL = "http://" + listener.Addr().String()
		for !wstuncli.IsConnected() {
			time.Sleep(10 * time.Millisecond)
		}
	})

	AfterEach(func() {
		wstuncli.Stop()
		wstunsrv.Stop()
		server.Close()
	})

	It("Times out requests to _token endpoint", func() {
		// Set up the server to delay slightly longer than the HTTP timeout
		server.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/delayed"),
				func(w http.ResponseWriter, r *http.Request) {
					// Sleep for 3 seconds, which should trigger the tunnel timeout
					time.Sleep(3 * time.Second)
					_, err := w.Write([]byte("This response should never be seen"))
					Ω(err).ShouldNot(HaveOccurred())
				},
			),
		)

		// Make a request that should time out
		client := &http.Client{
			Timeout: 5 * time.Second, // Client timeout to prevent hanging
		}

		start := time.Now()
		_, err := client.Get(wstunURL + "/_token/" + wstunToken + "/delayed")
		elapsed := time.Since(start)

		// Since the server timeout only applies to tunnel communication,
		// and the backend takes 3 seconds to respond, we expect a client timeout
		Ω(err).Should(HaveOccurred())
		Ω(err.Error()).Should(Or(
			ContainSubstring("timeout"),
			ContainSubstring("deadline exceeded"),
		))

		// The timeout should occur around the configured client timeout (5 seconds)
		// Allow some margin for processing time
		Ω(elapsed).Should(BeNumerically(">=", 4500*time.Millisecond))
		Ω(elapsed).Should(BeNumerically("<=", 5500*time.Millisecond))
	})
})
