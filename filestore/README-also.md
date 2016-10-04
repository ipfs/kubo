## Server side adds

**Note: Server side adds are currently disabled in the code due to
security concerns.  If you wish to enable this feature you will need
to compile IPFS from source and modify `repo/config/datastore.go`.**

When adding a file when the daemon is online.  The client sends both
the file contents and path to the server, and the server will then
verify that the same content is available via the specified path by
reading the file again on the server side.  To avoid this extra
overhead and allow directories to be added when the daemon is
online server side paths can be used.

To use this feature you must first enable API.ServerSideAdds using:
```
  ipfs config Filestore.APIServerSidePaths --bool true
```
*This option should be used with care since it will allow anyone with
access to the API Server access to any files that the daemon has
permission to read.* For security reasons it is probably best to only
enable this on a single user system and to make sure the API server is
configured to the default value of only binding to the localhost
(`127.0.0.1`).

With the `Filestore.APIServerSidePaths` option enabled you can add
files using `filestore add -S`.  For example, to add the file
`hello.txt` in the current directory use:
```
  ipfs filestore add -S -P hello.txt
```

