# Six Patches of Pain

Six Patches of Pain is an auto-updater for the **Super Clash of Ninja 4** mod. The name comes from the [Six Paths of Pain](https://naruto.fandom.com/wiki/Six_Paths_of_Pain).

- [How to Use](#how-to-use)
  - [Windows](#windows)
  - [Mac](#mac)
  - [Linux](#linux)
- [Common Questions](#common-questions)
- [Legal](#legal)

## How to Use

### Download the latest version

To download the latest version and patch it, run the program as specified below for your OS

#### Windows

Download the latest Windows release zip, extract it, and run `Six-Patches-Of-Pain.exe`

#### Mac

Download the latest Mac release zip and extract it. Then run Six Patches of Pain like so:

```bash
./Six-Patches-Of-Pain
```

#### Linux

Download the latest Linux release zip and extract it.
Then run Six Patches of Pain like so:

```bash
./Six-Patches-Of-Pain
```

### Download a specific version

To specify which available version to download and patch to run the program as specified below for your OS, and follow the on screen instructions

#### Windows

`Six-Patches-Of-Pain.exe -specific`

#### Mac

`./Six-Patches-Of-Pain -specific`

#### Linux

`./Six-Patches-Of-Pain -specific`

## Common Questions

### Why does it say my vanilla ISO needs to be modified?

When you create an ISO of a game, you read the data from the disc into a file. It is possible during
this process that a few bytes are different than expected. This is due to errors when reading the
disc. When the bytes differ, this is called a "bad dump" since it doesn't match the original game 1:1.
In many cases, bad dumps are perfectly fine because the bytes that changed are in places that are
unused. Furthermore, sometimes it is benefitial to create a bad dump, in order for the ISO to
compress better.

Six Patches of Pain expects that the GNT4 ISO used is a particular bad dump. A good dump uses random
bytes as padding on the disc, which is unable to be compressed and results in a larger zip size.
The bad dump used by Six Patches of Pain uses zeroes instead of random bytes so that it compresses
cleanly. When you use Six Patches of Pain with a good dump, it will ask you if you're okay with
modifying it to be the expected "bad dump".

When Six Patches of Pain asks to modify your good dump GNT4 ISO you have a few options. If you don't
care just hit enter and let it modify the file. If you are particularly concerned with keeping your
good dump ISO, consider creating a copy of it to be modified instead and use that with Six Patches
of Pain.

### Why Can I Not Use Nkit

Nkit ISOs are compressed versions of normal game ISOs. Six Patches of Pain expects a normal game ISO,
and therefore the Nkit ISO must be converted to a normal game ISO.

### It says I'm already on the latest version but I want to reinstall it

Open the `data` folder, delete the file named `current_version`, and restart Six Patches of Pain.

### How do I auto update from a different location (e.g. for betas)

First get the Github repository API URL to download it from, such as: https://api.github.com/repos/Super-GNT4/SCON4-Betas/releases

Run the executable with argument `<executable> -r <repository>`

For windows this would look like:

`six-patches-of-pain.exe -r https://api.github.com/repos/Super-GNT4/SCON4-Betas/releases`

###### Alternatively:

Make sure you've run Six Patches of Pain at least once. After you've do so, open the folder titled `data`. You should see a file titled `git_repository`. Right click on this file and select **Open with**. Select **Notepad**. Replace the URL in this file with the new URL and save the file.

You are now done, downloads will now come from this location instead of the previous.

![Example of using a different repository](different-repo-example.png?raw=true "Example of using a different repository")

## Building

To build the code, first make sure you have [go 1.16+](https://golang.org/) installed.

Then install `pb`, `goversioninfo` and `go-xdelta` by running:

```bash
make get
```

Finally, build the code with:

```bash
make <platform>
```

Currently Windows, Linux, and Mac are supported with respectively
```
windows
linux
mac
```


Different build environments can be targeted by using the `GOOS` env entry.

### Powershell Example

```powershell
$Env:GOOS = "windows"; $Env:GOARCH = "amd64"
make windows
$Env:GOOS = "linux"; $Env:GOARCH = "amd64"
make linux
$Env:GOOS = "darwin"; $Env:GOARCH = "amd64"
make mac
```

## Legal

This software is licensed under the GNU General Public License v3.0.

The icon for the application is owned by [thedemonknight](https://www.deviantart.com/thedemonknight/art/Naruto-dojutsu-icon-pack-270461865)
