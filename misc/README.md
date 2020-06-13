## init system integration

go-ipfs can be started by your operating system's native init system.

- [systemd](#systemd)
- [LSB init script](#initd)
- [Upstart/startup job](#upstart)
- [launchd](#launchd)

### systemd

For `systemd`, the best approach is to run the daemon in a user session. Here is a sample service file:

```systemd
[Unit]
Description=IPFS daemon

[Service]
# Environment="IPFS_PATH=/data/ipfs"  # optional path to ipfs init directory if not default ($HOME/.ipfs)
ExecStart=/usr/bin/ipfs daemon
Restart=on-failure

[Install]
WantedBy=default.target
```

To run this in your user session, save it as `~/.config/systemd/user/ipfs.service` (creating directories as necessary). Once you run `ipfs init` to create your IPFS settings, you can control the daemon using the following commands:

* `systemctl --user start ipfs` - start the daemon
* `systemctl --user stop ipfs` - stop the daemon
* `systemctl --user status ipfs` - get status of the daemon
* `systemctl --user enable ipfs` - enable starting the daemon at boot
* `systemctl --user disable ipfs` - disable starting the daemon at boot

*Note:* If you want this `--user` service to run at system boot, you must [`enable-linger`](http://www.freedesktop.org/software/systemd/man/loginctl.html) on the account that runs the service:

```
# loginctl enable-linger [user]
```
Read more about `--user` services here: [wiki.archlinux.org:Systemd ](https://wiki.archlinux.org/index.php/Systemd/User#Automatic_start-up_of_systemd_user_instances)

### initd

- Here is a full-featured sample service file: https://github.com/dylanPowers/ipfs-linux-service/blob/master/init.d/ipfs
- Use `service` or your distribution's equivalent to control the service.

##  upstart

- And below is a very basic sample upstart job. **Note the username jbenet**.

```
cat /etc/init/ipfs.conf
```
```
description "ipfs: interplanetary filesystem"

start on (local-filesystems and net-device-up IFACE!=lo)
stop on runlevel [!2345]

limit nofile 524288 1048576
limit nproc 524288 1048576
setuid jbenet
chdir /home/jbenet
respawn
exec ipfs daemon
```

Another version is available here:

```sh
ipfs cat /ipfs/QmbYCwVeA23vz6mzAiVQhJNa2JSiRH4ebef1v2e5EkDEZS/ipfs.conf >/etc/init/ipfs.conf
```

For both, edit to replace occurrences of `jbenet` with whatever user you want it to run as:

```sh
sed -i s/jbenet/<chosen-username>/ /etc/init/ipfs.conf
```

Once you run `ipfs init` to create your IPFS settings, you can control the daemon using the `init.d` commands:

```sh
sudo service ipfs start
sudo service ipfs stop
sudo service ipfs restart
...
```

## launchd

Similar to `systemd`, on macOS you can run `go-ipfs` via a user LaunchAgent.

- Create `~/Library/LaunchAgents/io.ipfs.go-ipfs.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
        <key>KeepAlive</key>
        <true/>
        <key>Label</key>
        <string>io.ipfs.go-ipfs</string>
        <key>ProcessType</key>
        <string>Background</string>
        <key>ProgramArguments</key>
        <array>
                <string>/bin/sh</string>
                <string>-c</string>
                <string>~/go/bin/ipfs daemon</string>
        </array>
        <key>RunAtLoad</key>
        <true/>
</dict>
</plist>
```
The reason for running `ipfs` under a shell is to avoid needing to hard-code the user's home directory in the job.

- To start the job, run `launchctl load ~/Library/LaunchAgents/io.ipfs.go-ipfs.plist`

Notes:

- To check that the job is running, run `launchctl list | grep ipfs`.
- IPFS should now start whenever you log in (and exit when you log out).
- [LaunchControl](http://www.soma-zone.com/LaunchControl/) is a GUI tool which simplifies management of LaunchAgents.## init system integration

go-ipfs can be started by your operating system's native init system.

- [systemd](#systemd)
- [LSB init script](#initd)
- [Upstart/startup job](#upstart)
- [launchd](#launchd)

### systemd

For `systemd`, the best approach is to run the daemon in a user session. Here is a sample service file:

```systemd
[Unit]
Description=IPFS daemon

[Service]
# Environment="IPFS_PATH=/data/ipfs"  # optional path to ipfs init directory if not default ($HOME/.ipfs)
ExecStart=/usr/bin/ipfs daemon
Restart=on-failure

[Install]
WantedBy=default.target
```

To run this in your user session, save it as `~/.config/systemd/user/ipfs.service` (creating directories as necessary). Once you run `ipfs init` to create your IPFS settings, you can control the daemon using the following commands:

* `systemctl --user start ipfs` - start the daemon
* `systemctl --user stop ipfs` - stop the daemon
* `systemctl --user status ipfs` - get status of the daemon
* `systemctl --user enable ipfs` - enable starting the daemon at boot
* `systemctl --user disable ipfs` - disable starting the daemon at boot

*Note:* If you want this `--user` service to run at system boot, you must [`enable-linger`](http://www.freedesktop.org/software/systemd/man/loginctl.html) on the account that runs the service:

```
# loginctl enable-linger [user]
```
Read more about `--user` services here: [wiki.archlinux.org:Systemd ](https://wiki.archlinux.org/index.php/Systemd/User#Automatic_start-up_of_systemd_user_instances)

### initd

- Here is a full-featured sample service file: https://github.com/dylanPowers/ipfs-linux-service/blob/master/init.d/ipfs
- Use `service` or your distribution's equivalent to control the service.

##  upstart

- And below is a very basic sample upstart job. **Note the username jbenet**.

```
cat /etc/init/ipfs.conf
```
```
description "ipfs: interplanetary filesystem"

start on (local-filesystems and net-device-up IFACE!=lo)
stop on runlevel [!2345]

limit nofile 524288 1048576
limit nproc 524288 1048576
setuid jbenet
chdir /home/jbenet
respawn
exec ipfs daemon
```

Another version is available here:

```sh
ipfs cat /ipfs/QmbYCwVeA23vz6mzAiVQhJNa2JSiRH4ebef1v2e5EkDEZS/ipfs.conf >/etc/init/ipfs.conf
```

For both, edit to replace occurrences of `jbenet` with whatever user you want it to run as:

```sh
sed -i s/jbenet/<chosen-username>/ /etc/init/ipfs.conf
```

Once you run `ipfs init` to create your IPFS settings, you can control the daemon using the `init.d` commands:

```sh
sudo service ipfs start
sudo service ipfs stop
sudo service ipfs restart
...
```

## launchd

Similar to `systemd`, on macOS you can run `go-ipfs` via a user LaunchAgent.

- Create `~/Library/LaunchAgents/io.ipfs.go-ipfs.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
        <key>KeepAlive</key>
        <true/>
        <key>Label</key>
        <string>io.ipfs.go-ipfs</string>
        <key>ProcessType</key>
        <string>Background</string>
        <key>ProgramArguments</key>
        <array>
                <string>/bin/sh</string>
                <string>-c</string>
                <string>~/go/bin/ipfs daemon</string>
        </array>
        <key>RunAtLoad</key>
        <true/>
</dict>
</plist>
```
The reason for running `ipfs` under a shell is to avoid needing to hard-code the user's home directory in the job.

- To start the job, run `launchctl load ~/Library/LaunchAgents/io.ipfs.go-ipfs.plist`

Notes:

- To check that the job is running, run `launchctl list | grep ipfs`.
- IPFS should now start whenever you log in (and exit when you log out).
- [LaunchControl](http://www.soma-zone.com/LaunchControl/) is a GUI tool which simplifies management of LaunchAgents.
