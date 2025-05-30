// Copyright (c) 2014 RightScale, Inc. - see LICENSE

package whois

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"regexp"
)

// DataPart host/body of data
type DataPart struct {
	Body string `json:"body"`
	Host string `json:"host"`
}

// DataResponse daystamp and parts
type DataResponse struct {
	Daystamp string     `json:"daystamp"`
	Parts    []DataPart `json:"parts"`
}

// Data is response body
type Data struct {
	Response DataResponse `json:"response"`
}

var orgNameRe = regexp.MustCompile("Org[^a-zA-Z]?Name[^a-zA-Z]*([ -~]*)")
var netNameRe = regexp.MustCompile("network:Organization[^a-zA-Z]*([ -~]*)")

// Whois determines ip information from robowhois
func Whois(ipAddr, apiToken string) string {
	result := ""
	if net.ParseIP(ipAddr) == nil {
		fmt.Printf("IP Address: %s - Invalid\n", ipAddr)
	} else {
		fmt.Printf("IP Address: %s - Valid\n", ipAddr)
		validIpAddr := net.ParseIP(ipAddr).String()
		url := fmt.Sprint("http://api.robowhois.com/v1/whois/", validIpAddr, "/parts")
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Printf("robowhois: error building URL for query for %s", validIpAddr)
		}
		req.SetBasicAuth(apiToken, "X")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("robowhois: query for %s failed: %s", validIpAddr, err)
		}
		if resp.StatusCode != 200 {
			log.Printf("robowhois: query for %s returned error: %s", validIpAddr, resp.Status)
		}
		var data Data
		err = json.NewDecoder(resp.Body).Decode(&data)
		if err != nil {
			log.Printf("robowhois: can't decode response for %s: %s", validIpAddr, err)
		}
		n := len(data.Response.Parts)
		//log.Printf("robowhois: %s -> %s", ipAddr, data.Response.Parts[n-1].Body)

		match := orgNameRe.FindAllStringSubmatch(data.Response.Parts[n-1].Body, -1)
		if match == nil {
			match = netNameRe.FindAllStringSubmatch(data.Response.Parts[n-1].Body, -1)
		}
		if match == nil {
			log.Printf("robowhois: can't find OrgName in response for %s", validIpAddr)
		}
		result = match[len(match)-1][1]
		log.Printf("robowhois: %s -> %s", validIpAddr, result)
	}
	return result
}

// func main() {
// 	fmt.Printf("%s -> %s\n", os.Args[1], Whois(os.Args[1], os.Args[2]))
// }
