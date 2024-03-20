# gorepo

The tool is intended for being used in the environment without Internet access,
where the [repo][1] tool can not be used. The tool aims for downloading [Yocto][2]
meta layers, thus only basic functionalities are implemented, which suffices
the needs when working with a Yocto project.

The way of how gorepo manages repositories is different from repo tool. It's more
like [west][3], which uses a local branch `manifest-rev` for recording revisions
specified in the manifest.

The tool is under development, and more features will be added.

## Requirement
* Go >= 1.21

## Installation
To intall the tool, just run the command:

    $ go install github.com/ramaxlo/gorepo@latest

## Usage
The tool can be a drop-in replacement of `repo`. For example, to download i.MX
Yocto BSP, run following command:

    $ gorepo init -u https://github.com/nxp-imx/imx-manifest -b imx-linux-nanbield -m imx-6.6.3-1.0.0.xml
    $ gorepo sync -j 4

To download MTK IoT Yocto BSP, run:

    $ repo init -u https://gitlab.com/mediatek/aiot/bsp/manifest.git -b refs/tags/rity-kirkstone-v23.2 -m default.xml
    $ gorepo sync -j 4

[1]: https://android.googlesource.com/tools/repo
[2]: https://www.yoctoproject.org
[3]: https://docs.zephyrproject.org/latest/develop/west/index.html
