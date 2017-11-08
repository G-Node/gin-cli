# Copyright (c) 2017, German Neuroinformatics Node (G-Node)
#
# All rights reserved.
#
# Redistribution and use in source and binary forms, with or without
# modification, are permitted under the terms of the BSD License. See
# LICENSE file in the root of the Project.
"""
Build gin-cli binaries and package them for distribution.
"""
import pickle
import sys
import os
import shutil
import json
import re
from glob import glob
from subprocess import check_output, call, DEVNULL
from tempfile import TemporaryDirectory
import requests
from requests.exceptions import ConnectionError as ConnError

DESTDIR = "dist"
PKGDIR = os.path.join(DESTDIR, "pkg")

ETAGFILE = os.path.join(DESTDIR, "etags")
ETAGS = {}  # type: dict

VERSION = {}


def load_etags():
    """
    Read in etags file and populates dictionary.
    """
    try:
        with open(ETAGFILE, "rb") as etagfile:
            ETAGS.update(pickle.load(etagfile))
    except FileNotFoundError:
        # print("--> No etags file found. Skipping load.")
        pass


def save_etags():
    """
    Save (potentially new) etags to file.
    """
    with open(ETAGFILE, "wb") as etagfile:
        pickle.dump(ETAGS, etagfile)


def download(url, fname=None):
    """
    Download a URL if necessary. If the URL's etag matches the existing one,
    the download is skipped.
    """
    if fname is None:
        fname = url.split("/")[-1]
    fname = os.path.join(DESTDIR, "downloads", fname)
    print("--> Downloading {} → {}".format(url, fname))
    try:
        req = requests.get(url, stream=True)
    except ConnError:
        print("Error while trying to download {}".format(url), file=sys.stderr)
        print("Skipping.", file=sys.stderr)
        return
    size = int(req.headers.get("content-length"))
    etag = req.headers.get("etag")
    oldet = ETAGS.get(url)
    if etag == oldet and os.path.exists(fname):
        fod_size = os.path.getsize(fname)
        if fod_size == size:
            print("File already downloaded. Skipping.", end="\n\n")
            return fname
    ETAGS[url] = etag
    prog = 0
    with open(fname, "wb") as dlfile:
        for chunk in req.iter_content(chunk_size=256):
            dlfile.write(chunk)
            prog += len(chunk)
            print("\r{:2.1f}%".format(prog / size * 100), end="", flush=True)
        print("\nDone!")
    print()
    return fname


def die(msg):
    """
    Exit the program with a given error message and exit status 1.
    """
    print(msg, file=sys.stderr)
    sys.exit(1)


def wait_for_ret():
    """
    Pause execution and wait for the user to hit return. If ctrl+c (interrupt)
    is received instead, exit the program with status 1.
    """
    try:
        input("Hit return to continue or ^C to cancel ...")
    except KeyboardInterrupt:
        die("\nCancelled")


def build():
    """
    Build binaries.
    """
    platforms = ["linux/amd64", "windows/386", "darwin/amd64"]
    print("--> Building binary for [{}]".format(", ".join(platforms)))
    verfilename = "version"
    with open(verfilename) as verfile:
        verinfo = verfile.read()

    VERSION["version"] = re.search(r"version=([0-9\.]+(dev){0,1})",
                                   verinfo).group(1)
    cmd = ["git", "rev-list", "--count", "HEAD"]
    VERSION["build"] = int(check_output(cmd).strip().decode())
    cmd = ["git", "rev-parse", "HEAD"]
    VERSION["commit"] = check_output(cmd).strip().decode()
    print(("Version: {version} "
           "Build: {build:06d} "
           "Commit: {commit}").format(**VERSION))
    ldflags = ("-X main.gincliversion={version} "
               "-X main.build={build:06d} "
               "-X main.commit={commit}").format(**VERSION)
    output = os.path.join(DESTDIR, "{{.OS}}-{{.Arch}}", "gin")
    cmd = [
        "gox", "-output={}".format(output), "-osarch={}".format(
            " ".join(platforms)), "-ldflags={}".format(ldflags)
    ]
    print("Running {}".format(" ".join(cmd)))
    if call(cmd) > 0:
        die("Build failed")

    print()
    print("--> Build succeeded")
    print("--> The following files were built:")
    ginfiles = glob(os.path.join(DESTDIR, "*", "gin"))
    ginfiles.extend(glob(os.path.join(DESTDIR, "*", "gin.exe")))
    print("\n".join(ginfiles), end="\n\n")

    plat = sys.platform
    for ginbin in ginfiles:
        if plat in ginbin:
            cmd = [ginbin, "--version"]
            verstring = check_output(cmd).strip().decode()
            print("{}\n↪ {}".format(" ".join(cmd), verstring))
    print()
    return ginfiles


def download_annex_sa():
    """
    Download annex standaline tarball.
    """
    annex_sa_url = ("https://downloads.kitenet.net/git-annex/linux/current/"
                    "git-annex-standalone-amd64.tar.gz")
    return download(annex_sa_url)


def get_git_for_windows():
    """
    Download the (portable) git for windows package.
    Relies on github API to find latest release.
    """
    url = "https://api.github.com/repos/git-for-windows/git/releases/latest"
    req = requests.get(url)
    releases = json.loads(req.text)
    assets = releases["assets"]
    for asset in assets:
        if "PortableGit" in asset["name"]:
            win_git_url = asset["browser_download_url"]
            break
    else:
        die("Could not find PortableGit download")
    return download(win_git_url)


def get_git_annex_for_windows():
    """
    Download the git annex for windows installer.
    """
    win_git_annex_url = ("https://downloads.kitenet.net/git-annex/windows/"
                         "current/git-annex-installer.exe")
    return download(win_git_annex_url)


def package_linux_plain(binfiles):
    """
    For each Linux binary make a tarball and include all related files.
    """
    archives = []
    for binf in binfiles:
        dirname, fname = os.path.split(binf)
        _, osarch = os.path.split(dirname)
        # simple binary archive
        shutil.copy("README.md", dirname)
        arc = "gin-cli-{}-{}.tar.gz".format(VERSION["version"], osarch)
        arc = os.path.join(PKGDIR, arc)
        cmd = ["tar", "-czf", arc, "-C", dirname, fname, "README.md"]
        print("Running {}".format(" ".join(cmd)))
        if call(cmd) > 0:
            print(f"Failed to make tarball for {binf}", file=sys.stderr)
            continue
        archives.append(arc)
    return archives


def debianize(binfiles, annexsa_archive):
    """
    For each Linux binary make a deb package with git annex standalone.
    """
    debs = []
    with TemporaryDirectory(suffix="gin-linux") as tmpdir:
        cmd = ["docker", "build", "-t", "gin-deb", "debdock/."]
        print("Preparing docker image for debian build")
        call(cmd)

        contdir = "/debbuild/"
        cmd = [
            "docker", "run", "-i", "-v", "{}:{}".format(tmpdir, contdir),
            "--name", "gin-deb-build", "-d", "gin-deb", "bash"
        ]
        print("Starting debian docker container")
        if call(cmd) > 0:
            print("Container start failed", file=sys.stderr)
            return

        cmd = ["docker", "ps"]
        print("docker ps")
        call(cmd)

        for binf in binfiles:
            # debian packaged with annex standalone
            # create directory structure
            # pkg gin-cli_version
            # /opt
            # /opt/gin/
            # /opt/gin/git-annex.linux/...
            # /opt/gin/bin/gin (binary)
            # /opt/gin/bin/gin.sh (shell script for running gin cmds)
            # /usr/local/gin -> /opt/gin/bin/gin.sh (symlink)

            # create directory structure
            pkgname = "gin-cli"
            pkgnamever = "{}-{}".format(pkgname, VERSION["version"])
            debmdsrc = os.path.join("debdock", "debian")
            pkgdir = os.path.join(tmpdir, pkgname)
            debcapdir = os.path.join(pkgdir, "DEBIAN")
            opt_dir = os.path.join(pkgdir, "opt")
            opt_gin_dir = os.path.join(opt_dir, "gin")
            opt_gin_bin_dir = os.path.join(opt_gin_dir, "bin")
            usr_local_bin_dir = os.path.join(pkgdir, "usr", "local", "bin")
            docdir = os.path.join(pkgdir, "usr", "share", "doc", pkgname)

            os.makedirs(debcapdir)
            os.makedirs(opt_gin_bin_dir)
            os.makedirs(usr_local_bin_dir)
            os.makedirs(docdir)

            # copy binaries and program files
            shutil.copy(binf, opt_gin_bin_dir)
            print(f"Copied {binf} to {opt_gin_bin_dir}")
            shutil.copy("gin.sh", opt_gin_bin_dir)
            print(f"Copied gin.sh to {opt_gin_bin_dir}")

            link_path = os.path.join(usr_local_bin_dir, "gin")
            os.symlink("/opt/gin/bin/gin.sh", link_path)

            shutil.copy("README.md", opt_gin_dir)

            # copy debian package metadata files
            shutil.copy(os.path.join(debmdsrc, "control"), debcapdir)
            shutil.copy("LICENSE", os.path.join(docdir, "copyright"))
            shutil.copy(os.path.join(debmdsrc, "changelog"), docdir)
            shutil.copy(os.path.join(debmdsrc, "changelog.Debian"), docdir)

            # TODO: Update changelog automatically
            # Adding version number to debian control file
            controlpath = os.path.join(debcapdir, "control")
            with open(controlpath) as controlfile:
                controllines = controlfile.read().format(**VERSION)

            with open(controlpath, "w") as controlfile:
                controlfile.write(controllines)

            # gzip changelog and changelog.Debian
            cmd = [
                "gzip", "--best",
                os.path.join(docdir, "changelog"),
                os.path.join(docdir, "changelog.Debian")
            ]
            if call(cmd) > 0:
                print(f"Failed to gzip files in {docdir}", file=sys.stderr)

            # extract annex standalone into pkg/opt/gin
            cmd = ["tar", "-xzf", annexsa_archive, "-C", opt_gin_dir]
            print("Running {}".format(" ".join(cmd)))
            if call(cmd) > 0:
                print("Failed to extract git annex standalone [{}]".format(
                    annexsa_archive, file=sys.stderr))
                continue

            dockerexec = ["docker", "exec", "-t", "gin-deb-build"]

            cmd = dockerexec + ["chmod", "go+rX,go-w", "-R", contdir]
            print("Fixing permissions for build dir")
            call(cmd)

            cmd = dockerexec + [
                "fakeroot", "dpkg-deb", "--build",
                os.path.join(contdir, pkgname)
            ]
            print("Building deb package")
            if call(cmd) > 0:
                print("Deb build failed", file=sys.stderr)
                continue

            debfilename = f"{pkgname}.deb"
            cmd = dockerexec + ["lintian", os.path.join(contdir, debfilename)]
            print("Running lintian on new deb file")
            if call(cmd, stdout=open(os.devnull, "wb")) > 0:
                print("Deb file check exited with errors")
                print("Ignoring for now")

            debfilepath = os.path.join(tmpdir, debfilename)
            debfiledest = os.path.join(PKGDIR, f"{pkgnamever}.deb")
            if os.path.exists(debfiledest):
                os.remove(debfiledest)
            shutil.copy(debfilepath, debfiledest)
            debs.append(debfiledest)
            print("Done")
        print("Stopping and cleaning up docker container")
        cmd = ["docker", "kill", "gin-deb-build"]
        call(cmd)
        cmd = ["docker", "container", "rm", "gin-deb-build"]
        call(cmd)
    return debs


def rpmify(binfiles, annexsa_archive):
    """
    For each Linux binary make a rpm package with git annex standalone.
    """
    return []


def package_mac_plain(binfiles):
    """
    For each Darwin binary make a tarball and include all related files.
    """
    archives = []
    for binf in binfiles:
        dirname, fname = os.path.split(binf)
        _, osarch = os.path.split(dirname)
        osarch = osarch.replace("darwin", "macos")
        # simple binary archive
        shutil.copy("README.md", dirname)
        arc = "gin-cli-{}-{}.tar.gz".format(VERSION["version"], osarch)
        arc = os.path.join(PKGDIR, arc)
        cmd = ["tar", "-czf", arc, "-C", dirname, fname, "README.md"]
        print("Running {}".format(" ".join(cmd)))
        if call(cmd) > 0:
            print(f"Failed to make tarball for {binf}", file=sys.stderr)
            continue
        archives.append(arc)
    return archives


def winbundle(binfiles, git_pkg, annex_pkg):
    """
    For each Windows binary make a zip and include git and git annex portable
    """
    winarchives = []
    for binf in binfiles:
        with TemporaryDirectory(suffix="gin-windows") as tmpdir:
            pkgroot = os.path.join(tmpdir, "gin")
            bindir = os.path.join(pkgroot, "bin")
            os.makedirs(bindir)

            shutil.copy(binf, bindir)
            shutil.copy("README.md", pkgroot)
            shutil.copy("gin-shell.bat", pkgroot)

            gitdir = os.path.join(pkgroot, "git")
            os.makedirs(gitdir)

            # extract git portable and annex into git dir
            cmd = ["7z", "x", "-o{}".format(gitdir), git_pkg]
            print("Running {}".format(" ".join(cmd)))
            if call(cmd, stdout=DEVNULL) > 0:
                print(
                    "Failed to extract git archive [{}]".format(git_pkg),
                    file=sys.stderr)
                continue

            cmd = ["7z", "x", "-o{}".format(gitdir), annex_pkg]
            print("Running {}".format(" ".join(cmd)))
            if call(cmd, stdout=DEVNULL) > 0:
                print(
                    "Failed to extract git archive [{}]".format(annex_pkg),
                    file=sys.stderr)
                continue
            dirname, _ = os.path.split(binf)
            _, osarch = os.path.split(dirname)

            arc = "gin-cli-{}-{}.zip".format(VERSION["version"], osarch)
            arc = os.path.join(PKGDIR, arc)
            print("Creating Windows zip file")
            # need to change paths before making zip file
            if os.path.exists(arc):
                os.remove(arc)
            arc_abs = os.path.abspath(arc)
            oldwd = os.getcwd()
            os.chdir(pkgroot)
            cmd = ["zip", "-r", arc_abs, "."]
            print("Running {} (from {})".format(" ".join(cmd), pkgroot))
            if call(cmd, stdout=DEVNULL) > 0:
                print(
                    "Failed to create archive [{}]".format(arc),
                    file=sys.stderr)
                os.chdir(oldwd)
                continue
            os.chdir(oldwd)
            winarchives.append(arc)
            print("DONE")
    return winarchives


def main():
    """
    Main
    """
    os.makedirs(os.path.join(DESTDIR, "downloads"), exist_ok=True)
    os.makedirs(PKGDIR, exist_ok=True)

    binfiles = build()
    load_etags()
    annexsa_file = download_annex_sa()
    win_git_file = get_git_for_windows()
    win_git_annex_file = get_git_annex_for_windows()
    save_etags()

    print("Ready to package")

    linux_bins = [b for b in binfiles if "linux" in b]
    win_bins = [b for b in binfiles if "windows" in b]
    darwin_bins = [b for b in binfiles if "darwin" in b]

    linux_pkgs = package_linux_plain(linux_bins)
    deb_pkgs = debianize(linux_bins, annexsa_file)

    rpm_pkgs = rpmify(linux_bins, annexsa_file)

    mac_pkgs = package_mac_plain(darwin_bins)

    win_pkgs = winbundle(win_bins, win_git_file, win_git_annex_file)

    def printlist(lst):
        """
        Print a list of files.
        """
        print("".join("> " + l + "\n" for l in lst))

    def link_latest(lst):
        """
        Create symlinks with the version part replaced by 'latest' for the
        newly built packages.
        """
        for fname in lst:
            latestname = fname.replace(VERSION["version"], "latest")
            print("Linking {} to {}".format(fname, latestname))
            if os.path.exists(latestname):
                os.unlink(latestname)
            os.link(fname, latestname)

    print("------------------------------------------------")
    print("The following archives and packages were created")
    print("------------------------------------------------")
    print("Linux tarballs:")
    printlist(linux_pkgs)
    link_latest(linux_pkgs)

    print("Debian packages:")
    printlist(deb_pkgs)
    link_latest(deb_pkgs)

    print("RPM packages:")
    printlist(rpm_pkgs)
    link_latest(rpm_pkgs)

    print("macOS packages:")
    printlist(mac_pkgs)
    link_latest(mac_pkgs)

    print("Windows packages:")
    printlist(win_pkgs)
    link_latest(win_pkgs)


if __name__ == "__main__":
    main()
