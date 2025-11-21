# systemd cheat sheet

Common commands to manage the deployed services on the EC2 host. Run them with `sudo` on the box where the units were installed.

## react-client service
- Restart frontend: `sudo systemctl restart react-client`
- Check status: `sudo systemctl status react-client`
- Follow logs: `sudo journalctl -u react-client -f`

## app service
- Restart backend: `sudo systemctl restart app`
- Check status: `sudo systemctl status app`
- Follow logs: `sudo journalctl -u app -f`

## directory service
- Restart directory: `sudo systemctl restart directory`
- Check status: `sudo systemctl status directory`
- Follow logs: `sudo journalctl -u directory -f`

## redis service
- Restart redis: `sudo systemctl restart redis`
- Check status: `sudo systemctl status redis`
- Follow logs: `sudo journalctl -u redis -f`

General helpers
- Reload units after editing: `sudo systemctl daemon-reload`
- List failed units: `systemctl --failed`
- Enable service on boot: `sudo systemctl enable <unit>`
- Disable service on boot: `sudo systemctl disable <unit>`
