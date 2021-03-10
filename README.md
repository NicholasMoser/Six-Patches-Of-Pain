# Six Patches of Pain

Six Patches of Pain is an auto-updater for the **Super Clash of Ninja 4** mod. The name comes from the [Six Paths of Pain](https://naruto.fandom.com/wiki/Six_Paths_of_Pain).

- [How to Use](#how-to-use)
  - [Windows](#windows)
  - [Mac](#mac)
  - [Linux](#linux)
- [Common Questions](#common-questions)
- [Legal](#legal)

## How to Use

### Windows

Download the latest Windows release zip, extract it, and run `six_patches_of_pain.exe`

### Mac

Download the latest Mac release zip and extract it. Then make sure you have the following installed:

- Homebrew
  - Download and install from https://brew.sh/
- xdelta
  - To install run `brew install xdelta`

Then run Six Patches of Pain like so:

```bash
./six_patches_of_pain
```

### Linux

Download the latest Linux release zip and extract it. Then make sure you have the following installed:

- xdelta3
  - To install run `sudo apt-get install xdelta3`

Then run Six Patches of Pain like so:

```bash
./six_patches_of_pain
```

## Common Questions

### It says I'm already on the latest version but I want to reinstall it

Open the `data` folder, delete the file named `current_version`, and restart Six Patches of Pain.

## Building

To build the code, first make sure you have [go 1.16+](https://golang.org/) installed.

Then install `pb` and `goversioninfo` by running:

```bash
go get github.com/cheggaaa/pb/v3
go get github.com/josephspurrier/goversioninfo/cmd/goversioninfo
```

Finally, build the code with:

```bash
go generate
go build
```

Different build environments can be targeted by using the `GOOS` env entry.

### Powershell Example

```powershell
$Env:GOOS = "windows"; $Env:GOARCH = "amd64"
go generate
go build
$Env:GOOS = "linux"; $Env:GOARCH = "amd64"
go build
$Env:GOOS = "darwin"; $Env:GOARCH = "amd64"
go build
```

## Legal

This software is licensed under the GNU General Public License v3.0.

The bundled xdelta for Windows is licensed under Apache Public License version 2.0.

The icon for the application is owned by [thedemonknight](https://www.deviantart.com/thedemonknight/art/Naruto-dojutsu-icon-pack-270461865)
