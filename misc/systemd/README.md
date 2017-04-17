systemd
-------

**Warning:** This is a `--user` service, do not copy to `/etc/systemd/system/` or you will end up in running this as root.

To start ipfs with systemd, run `mkdir -p ~/.config/systemd/user && cp ipfs.service ~/.config/systemd/user/`.

After you've created your ipfs folder with `ipfs init` you can control the daemon with the following commands:

- `systemctl --user start ipfs` - start the daemon
- `systemctl --user stop ipfs` - stop the daemon
- `systemctl --user status ipfs` - get status of the daemon
- `systemctl --user enable ipfs` - enables the daemon at boot
- `systemctl --user disable ipfs` - disables the daemon at boot

If you haven't already enabled `--user` services to run at boot, you have to [`enable-linger`][1] on the account that runs the service:

```
# loginctl enable-linger [user]
```

Read more about `--user` services: [wiki.archlinux.org:Systemd ][2]

[1]: http://www.freedesktop.org/software/systemd/man/loginctl.html
[2]: https://wiki.archlinux.org/index.php/Systemd/User#Automatic_start-up_of_systemd_user_instances
