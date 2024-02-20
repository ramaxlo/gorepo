# gorepo -- A stripped-down version of repo tool, written in Go

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

    $ go install github.com/ramax/gorepo

[1]: https://android.googlesource.com/tools/repo
[2]: https://www.yoctoproject.org
[3]: https://docs.zephyrproject.org/latest/develop/west/index.html
