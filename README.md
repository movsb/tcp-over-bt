# TCP-OVER-BT

Make a TCP connection from Bluetooth.

## Use Case

SSH into a headless Raspberry Pi (e.g.: Zero 2 W) which has a bad network configuration.

## For Device

When you have network connection:

1. Run `make device` to build the binary for device.
2. Put the binary into `/usr/local/bin`.
3. Move the `.service` file into `/etc/systemd/system`.
4. Enable: `sudo systemctl daemon-reload && sudo systemctl enable tcp-over-bt.service`

## For Host

1. Run `make host`
2. Create a SSH config in `~/.ssh/config`:

   ```ssh_config
   Host zero
   	ProxyCommand tcp-over-bt
   ```

3. `ssh zero` and wait patiently, it will discover the first available device.

**Note:** Only one SSH connection is allowed at a time, which is a characteristic of Bluetooth connections.
If you want multiple shells, use *tmux*.

## Security

There's no Bluetooth Paring. Anyone can connect to the same device. Authentication is done by SSH.
