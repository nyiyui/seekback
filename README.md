# Seekback

Seekback is a tool that runs in the background, continuously recording the past 20 seconds (configurable) of audio from your mic. When you hit a button, it will take those previous 20 seconds, and save it to a timestamped file.

I've had many times where I forgot what someone said a few seconds ago. This program is supposed to replay those last 20 or so seconds, and help me remember what they said.

Note that this is supposed to be run by systemd (e.g. using `StandardInput=socket`) so you can configure a keybind to send `\n` to the socket.

## Building

`portaudio-2.0`/`portaudio` and `pkg-config` is required. Run `go build ./cmd/seekback` to build.

You can also build using Nix flakes: `nix build`
