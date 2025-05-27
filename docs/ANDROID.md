# Using WStunnel on Android

WStunnel can be run on Android devices using a terminal emulator. This guide explains how to set up and use WStunnel as a client on Android.

## Prerequisites

1. **Terminal Emulator**: Install a terminal app from Google Play Store:
   - [Termux](https://termux.com/) (Recommended)
   - [Terminal Emulator for Android](https://play.google.com/store/apps/details?id=jackpal.androidterm)

2. **Root Access**: Not required for basic usage, but may be needed for certain network operations.

## Installation Methods

### Method 1: Download Pre-built Binary (Recommended)

1. Open your terminal emulator (e.g., Termux)

2. Download the Android ARM64 binary:
   ```bash
   # For ARM64 devices (most modern phones)
   wget https://github.com/rshade/wstunnel/releases/latest/download/wstunnel_1.1.1_android_arm64.tar.gz
   
   # Extract the binary
   tar -xzf wstunnel_1.1.1_android_arm64.tar.gz
   
   # Make it executable
   chmod +x wstunnel
   ```

3. Move to a directory in PATH (optional):
   ```bash
   mkdir -p $HOME/bin
   mv wstunnel $HOME/bin/
   export PATH=$HOME/bin:$PATH
   ```

### Method 2: Build from Source

If you have Go installed on your Android device (via Termux):

```bash
# Install dependencies in Termux
pkg update
pkg install golang git

# Clone and build
git clone https://github.com/rshade/wstunnel.git
cd wstunnel
go build -o wstunnel .
```

## Usage

### Basic Client Setup

1. Connect to your WStunnel server:
   ```bash
   ./wstunnel cli \
     -tunnel ws://your-server.com:8080 \
     -server http://localhost:8080 \
     -token 'your_secret_token'
   ```

2. For secure connections (WSS):
   ```bash
   ./wstunnel cli \
     -tunnel wss://your-server.com:443 \
     -server http://localhost:8080 \
     -token 'your_secret_token'
   ```

### Running in Background

To keep WStunnel running when you close the terminal:

```bash
# Using nohup
nohup ./wstunnel cli -tunnel ws://server:8080 -server http://localhost:8080 -token 'token' &

# Or using Termux's wake-lock to prevent Android from killing the process
termux-wake-lock
./wstunnel cli -tunnel ws://server:8080 -server http://localhost:8080 -token 'token'
```

### Auto-start on Boot (Termux)

1. Create a startup script:
   ```bash
   mkdir -p ~/.termux/boot/
   cat > ~/.termux/boot/start-wstunnel.sh << 'EOF'
   #!/data/data/com.termux/files/usr/bin/sh
   termux-wake-lock
   $HOME/bin/wstunnel cli \
     -tunnel ws://your-server:8080 \
     -server http://localhost:8080 \
     -token 'your_token' \
     >> $HOME/wstunnel.log 2>&1
   EOF
   chmod +x ~/.termux/boot/start-wstunnel.sh
   ```

2. Install Termux:Boot app from F-Droid to enable boot scripts.

## Connecting Through Android Apps

Once WStunnel is running, you can use it with Android apps that support HTTP proxies:

1. **Local Proxy Setup**: Run WStunnel to forward to a local proxy port
2. **Configure Apps**: Set your apps to use `localhost` and the configured port as HTTP proxy

## Battery Optimization

Android may kill background processes to save battery. To prevent this:

1. **Termux**: Use `termux-wake-lock` command
2. **Battery Settings**: Exclude your terminal app from battery optimization
3. **Persistent Notification**: Some terminal apps can show a persistent notification to stay alive

## Troubleshooting

### Permission Denied
If you get permission errors:
```bash
chmod +x wstunnel
```

### Network Issues
- Ensure your Android device has network connectivity
- Check if your carrier blocks WebSocket connections
- Try using WSS (secure WebSocket) instead of WS

### Process Killed
If Android keeps killing the process:
- Use `termux-wake-lock` in Termux
- Disable battery optimization for the terminal app
- Consider using a VPN app that supports custom protocols instead

## Alternative: GUI Wrapper

For users who prefer a graphical interface, you could:
1. Use automation apps like Tasker or Automate to create a GUI wrapper
2. Create shortcuts using terminal launcher apps
3. Use SSH clients that support running commands on connect

## Security Notes

- Store your token securely, don't hardcode it in scripts
- Use WSS (WebSocket Secure) when possible
- Be aware that some networks may inspect WebSocket traffic
- Consider using a VPN in addition to WStunnel for extra security

## Limitations

- No native Android GUI (command-line only)
- Battery usage may be higher due to persistent connection
- Some Android versions may aggressively kill background processes
- Root access may be required for binding to privileged ports

For more information, see the main [README](../README.md).