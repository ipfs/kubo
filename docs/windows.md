# Building on Windows
![](https://ipfs.io/ipfs/QmccXW7JSZMVXidSc7tHsU6aktuaiV923q4yBGHUsdymYo/build.gif)

If you just want to install kubo, please download it from https://dist.ipfs.tech/#kubo. This document explains how to build it from source.

## Install Go
`kubo` is built on Golang and thus depends on it for all building methods.  
https://golang.org/doc/install  
The `GOPATH` environment variable must be set as well.  
https://golang.org/doc/code.html#GOPATH

## Choose the way you want to proceed
`kubo` utilizes `make` to automate builds and run tests, but can be built without it using only `git` and `go`.  
No matter which method you choose, if you encounter issues, please see the [Troubleshooting](#troubleshooting) section.  

**Using `make`:**  
MSYS2 and Cygwin provide the Unix tools we need to build `kubo`. You may use either, but if you don't already have one installed, we recommend MSYS2.  
[MSYS2→](#msys2)  
[Cygwin→](#cygwin)  

**Using build tools manually:**  
This section assumes you have a working version of `go` and `git` already setup. You may want to build this way if your environment restricts installing additional software, or if you're integrating IPFS into your own build system.  
[Minimal→](#minimal)  

## MSYS2
1. Install msys2 (http://www.msys2.org)  
2. Run the following inside a normal `cmd` prompt (Not the MSYS2 prompt, we only need MSYS2's tools).  
An explanation of this block is below.
```
SET PATH=%PATH%;\msys64\usr\bin
pacman --noconfirm -S  git make unzip
go get -u github.com/ipfs/kubo
cd %GOPATH%\src\github.com\ipfs\kubo
make install
%GOPATH%\bin\ipfs.exe version --all
```

If there were no errors, the final command should output version information similar to "`ipfs version 0.4.14-dev-XXXXXXX`" where "XXXXXXX" should match the current short-hash of the `kubo` repo. You can retrieve said hash via this command: `git rev-parse --short HEAD`.  
If `ipfs.exe` executes and the version string matches, then building was successful.

|Command|Explanation|
| ---: | :--- |
|`SET PATH=%PATH%;\msys64\usr\bin`         |Add msys2's tools to our [`PATH`](https://ss64.com/nt/path.html); Defaults to: (\msys64\usr\bin)|
|`pacman --noconfirm -S  git make unzip`   |Install `kubo` build dependencies|
|`go get -u github.com/ipfs/kubo`       |Fetch / Update `kubo` source|
|`cd %GOPATH%\src\github.com\ipfs\kubo` |Change to `kubo` source directory|
|`make install`                            |Build and install to `%GOPATH%\bin\ipfs.exe`|
|`%GOPATH%\bin\ipfs.exe version --all`     |Test the built binary|

To build again after making changes to the source, run:
```
SET PATH=%PATH%;\msys64\usr\bin
cd %GOPATH%\src\github.com\ipfs\kubo
make install
```

**Tip:** To avoid setting `PATH` every time (`SET PATH=%PATH%;\msys64\usr\bin`), you can lock it in permanently using `setx` after it's been set once:
```
SETX PATH %PATH%
```

## Cygwin
1. Install Cygwin (https://www.cygwin.com)  
2. During the install, select the following packages. (If you already have Cygwin installed, run the setup file again to install additional packages.) A fresh install should look something like [this reference image](https://ipfs.io/ipfs/QmaYFSQa4iHDafcebiKjm1WwuKhosoXr45HPpfaeMbCRpb/cygwin%20-%20install.png).
    - devel packages
        - `git`
        - `make`
    - archive packages
        - `unzip`
    - net packages
        - `curl`  
3. Run the following inside a normal `cmd` prompt (Not the Cygwin prompt, we only need Cygwin's tools)  
An explanation of this block is below.
```
SET PATH=%PATH%;\cygwin64\bin
mkdir %GOPATH%\src\github.com\ipfs
cd %GOPATH%\src\github.com\ipfs
git clone https://github.com/ipfs/kubo.git
cd %GOPATH%\src\github.com\ipfs\kubo
make install
%GOPATH%\bin\ipfs.exe version --all
```

If there were no errors, the final command should output version information similar  to "`ipfs version 0.4.14-dev-XXXXXXX`" where "XXXXXXX" should match the current short-hash of the `kubo` repo. You can retrieve said hash via this command: `git rev-parse --short HEAD`.  
If `ipfs.exe` executes and the version string matches, then building was successful.

|Command|Explanation|
| ---: | :--- |
|`SET PATH=%PATH%;\cygwin64\bin`           |Add Cygwin's tools to our [`PATH`](https://ss64.com/nt/path.html); Defaults to: (\cygwin64\bin)|
|`mkdir %GOPATH%\src\github.com\ipfs`<br/>`cd %GOPATH%\src\github.com\ipfs`<br/>`git clone https://github.com/ipfs/kubo.git`       |Fetch / Update `kubo` source|
|`cd %GOPATH%\src\github.com\ipfs\kubo` |Change to `kubo` source directory|
|`make install`                            |Build and install to `%GOPATH%\bin\ipfs.exe`|
|`%GOPATH%\bin\ipfs.exe version --all`     |Test the built binary|

To build again after making changes to the source, run:
```
SET PATH=%PATH%;\cygwin64\bin
cd %GOPATH%\src\github.com\ipfs\kubo
make install
```

**Tip:** To avoid setting `PATH` every time (`SET PATH=%PATH%;\cygwin64\bin`), you can lock it in permanently using `setx` after it's been set once:
```
SETX PATH %PATH%
```

## Minimal

While it's possible to build `kubo` with `go` alone, we'll be using `git` to fetch the source.

You can use whichever version of `git` you wish but we recommend the Windows builds at <https://git-scm.com>. `git` must be in your [`PATH`](https://ss64.com/nt/path.html) for `go get` to recognize and use it.

### kubo

Clone and change directory to the source code, if you haven't already:

CMD:
```bat
git clone https://github.com/ipfs/kubo %GOPATH%/src/github.com/ipfs/kubo
cd %GOPATH%/src/github.com/ipfs/kubo/cmd/ipfs
```

PowerShell:
```powershell
git clone https://github.com/ipfs/kubo $env:GOPATH/src/github.com/ipfs/kubo
cd $env:GOPATH/src/github.com/ipfs/kubo/cmd/ipfs
```

We need the `git` commit hash to be included in our build so that in the extremely rare event a bug is found, we have a reference point later for tracking it. We'll ask `git` for it and store it in a variable. The syntax for the next command is different depending on whether you're using the interactive command line or writing a batch file. Use the one that applies to you.  
- interactive: `FOR /F %V IN ('git rev-parse --short HEAD') do set SHA=%V`  
- interpreter: `FOR /F %%V IN ('git rev-parse --short HEAD') do set SHA=%%V`  

Finally, we'll build and test `ipfs` itself.

CMD:
```bat
go install -ldflags="-X "github.com/ipfs/kubo".CurrentCommit=%SHA%"
%GOPATH%\bin\ipfs.exe version --all
```

PowerShell:
```powershell
go install -ldflags="-X "github.com/ipfs/kubo".CurrentCommit=$env:SHA"
cp ./ipfs.exe $env:GOPATH/bin/ipfs.exe -force
. $env:GOPATH/bin/ipfs.exe version --all
```
You can check that the ipfs output versions match with `go version` and `git rev-parse --short HEAD`.  
If `ipfs.exe` executes and everything matches, then building was successful.

## Troubleshooting
- **Git auth**
If you get authentication problems with Git, you might want to take a look at https://help.github.com/articles/caching-your-github-password-in-git/ and use the suggested solution:  
`git config --global credential.helper wincred`

- **Anything else**  
Please search [https://discuss.ipfs.io](https://discuss.ipfs.io/search?q=windows%20category%3A13) for any additional issues you may encounter. If you can't find any existing resolution, feel free to post a question asking for help.

If you encounter a bug with `kubo` itself (not related to building) please use the [issue tracker](https://github.com/ipfs/kubo/issues) to report it.
