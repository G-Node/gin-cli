# -*- coding: utf-8 -*-
import pickle
import sys
import os
import shutil
import json
import re
from glob import glob
from subprocess import check_output, call, DEVNULL
import requests
from requests.exceptions import ConnectionError
from tempfile import TemporaryDirectory

destdir = "dist"
pkgdir = os.path.join(destdir, "pkg")

etagfile = os.path.join(destdir, "etags")
etags = {}

version = {}


def load_etags():
    try:
        with open(etagfile, "rb") as fd:
            etags.update(pickle.load(fd))
    except FileNotFoundError:
        # print("--> No etags file found. Skipping load.")
        pass


def save_etags():
    with open(etagfile, "wb") as fd:
        pickle.dump(etags, fd)


def download(url, fname=None):
    if fname is None:
        fname = url.split("/")[-1]
    fname = os.path.join(destdir, "downloads", fname)
    print("--> Downloading {} → {}".format(url, fname))
    try:
        req = requests.get(url, stream=True)
    except ConnectionError:
        print("Error while trying to download {}".format(url),
              file=sys.stderr)
        print("Skipping.", file=sys.stderr)
        return
    size = int(req.headers.get("content-length"))
    et = req.headers.get("etag")
    oldet = etags.get(url)
    if et == oldet and os.path.exists(fname):
        fod_size = os.path.getsize(fname)
        if fod_size == size:
            print("File already downloaded. Skipping.", end="\n\n")
            return fname
    etags[url] = et
    prog = 0
    with open(fname, "wb") as fd:
        for chunk in req.iter_content(chunk_size=256):
            fd.write(chunk)
            prog += len(chunk)
            print("\r{:2.1f}%".format(prog/size*100), end="", flush=True)
        print("\nDone!")
    print()
    return fname


def die(msg):
    print(msg, file=sys.stderr)
    sys.exit(1)


def wait_for_ret():
    try:
        input("Hit return to continue or ^C to cancel ...")
    except KeyboardInterrupt:
        die("\nCancelled")


def build(incbuild=False):
    platforms = ["linux/amd64", "windows/386", "darwin/amd64"]
    print("--> Building binary for [{}]".format(", ".join(platforms)))
    verfile = "version"
    with open(verfile) as fd:
        verinfo = fd.read()

    version["version"] = re.search(r"version=([v0-9\.]+)", verinfo).group(1)
    version["build"] = re.search(r"build=([0-9]+)", verinfo).group(1)
    cmd = ["git", "rev-parse", "HEAD"]
    version["commit"] = check_output(cmd).strip().decode()
    if incbuild:
        print("--> Updating version file")
        version["build"] = "{:05d}".format(int(version["build"]) + 1)
        newinfo = "version={version}\nbuild={build}\n".format(**version)
        print(newinfo)
        with open(verfile, "w") as fd:
            fd.write(newinfo)
    print(("Version: {version} "
           "Build: {build} "
           "Commit: {commit}").format(**version))
    ldflags = ("-X main.version={version} "
               "-X main.build={build} "
               "-X main.commit={commit}").format(**version)
    # cmd = ["go", "build", "-ldflags", ldflags, "-o", "gin"]
    output = os.path.join(destdir, "{{.OS}}-{{.Arch}}", "gin")
    cmd = ["gox", "-output={}".format(output),
           "-osarch={}".format(" ".join(platforms)),
           "-ldflags={}".format(ldflags)]
    print("Running {}".format(" ".join(cmd)))
    ret = call(cmd)
    print()
    if ret > 0:
        die("Build failed")

    print("--> Build succeeded")
    print("--> The following files were built:")
    ginfiles = glob(os.path.join(destdir, "*", "gin"))
    ginfiles.extend(glob(os.path.join(destdir, "*", "gin.exe")))
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
    annex_sa_url = ("https://downloads.kitenet.net/git-annex/linux/current/"
                    "git-annex-standalone-amd64.tar.gz")
    return download(annex_sa_url)


def get_appveyor_artifact_url():
    """
    Queries Appveyor for the latest job artifacts. Returns the URL for the
    latest 32bit binary only.
    """
    apiurl = "https://ci.appveyor.com/api/"
    account = "achilleas-k"
    project_name = "gin-cli"

    url = os.path.join(apiurl, "projects", account, project_name)
    r = requests.get(url)

    projects = json.loads(r.text)
    build = projects["build"]
    for job in build["jobs"]:
        if job["status"] == "success":
            artifacts_url = os.path.join(apiurl, "buildjobs", job["jobId"],
                                         "artifacts")
            r = requests.get(artifacts_url)
            artifacts = json.loads(r.text)
            if "ARCH=32" in job["name"]:
                a = artifacts[0]
                arturl = os.path.join(apiurl, "buildjobs", job["jobId"],
                                      "artifacts", a["fileName"])
                return arturl


def get_git_for_windows():
    win_git_url = ("https://github.com/git-for-windows/git/releases/download/"
                   "v2.12.0.windows.1/PortableGit-2.12.0-32-bit.7z.exe")
    return download(win_git_url)


def get_git_annex_for_windows():
    win_git_annex_url = ("https://downloads.kitenet.net/git-annex/windows/"
                         "current/git-annex-installer.exe")
    return download(win_git_annex_url)


def package_linux_plain(binfiles):
    """
    For each Linux binary make a tarball and include all related files
    """
    archives = []
    for bf in binfiles:
        d, f = os.path.split(bf)
        _, osarch = os.path.split(d)
        # simple binary archive
        shutil.copy("README.md", d)
        arc = "gin-cli_{}-{}.tar.gz".format(version["version"], osarch)
        arc = os.path.join(pkgdir, arc)
        cmd = ["tar", "-czf", arc, "-C", d, f, "README.md"]
        print("Running {}".format(" ".join(cmd)))
        ret = call(cmd)
        if ret > 0:
            print("Failed to make tarball for {}".format(bf), file=sys.stderr)
            continue
        archives.append(arc)
    return archives


def debianize(binfiles, annexsa_archive):
    """
    For each Linux binary make a deb package with git annex standalone
    """
    debs = []
    for bf in binfiles:
        # debian packaged with annex standalone
        with TemporaryDirectory(suffix="gin-linux") as tmp_dir:
            # create directory structure
            # pkg gin-cli_version
            # /opt
            # /opt/gin/
            # /opt/gin/git-annex.linux/...
            # /opt/gin/bin/gin (binary)
            # /opt/gin/bin/gin.sh (shell script for running gin cmds)
            # /usr/local/gin -> /opt/gin/bin/gin.sh (symlink)

            pkgname = "gin-cli_{}".format(version["version"])
            build_dir = os.path.join(tmp_dir, pkgname)
            opt_dir = os.path.join(build_dir, "opt")
            opt_gin_dir = os.path.join(opt_dir, "gin")
            opt_gin_bin_dir = os.path.join(opt_gin_dir, "bin")
            os.makedirs(opt_gin_bin_dir)
            usr_local_bin_dir = os.path.join(build_dir, "usr", "local", "bin")
            os.makedirs(usr_local_bin_dir)

            shutil.copy(bf, opt_gin_bin_dir)
            shutil.copy("gin.sh", opt_gin_bin_dir)

            link_path = os.path.join(usr_local_bin_dir, "gin")
            os.symlink("/opt/gin/bin/gin.sh", link_path)

            # extract annex standalone into pkg/opt/gin
            cmd = ["tar", "-xzf", annexsa_archive, "-C", opt_gin_dir]
            print("Running {}".format(" ".join(cmd)))
            ret = call(cmd)
            if ret > 0:
                print("Failed to extract git annex standalone [{}]".format(
                    annexsa_archive, file=sys.stderr
                ))
                continue

            shutil.copytree("debdock/DEBIAN",
                            os.path.join(build_dir, "DEBIAN"))
            shutil.copy("README.md", opt_gin_dir)

            cmd = ["docker", "build",
                   "-t", "gin-deb", "debdock/."]
            print("Preparing docker image for debian build")
            ret = call(cmd)
            if ret > 0:
                print("Docker build failed", file=sys.stderr)
                continue

            cmd = ["docker", "run", "-v", "{}:/debbuild/".format(tmp_dir),
                   "gin-deb", "/bin/bash", "-c",
                   ("chown root:root -R /debbuild &&"
                    " chmod go+rX,go-w -R /debbuild")]
            print("Fixing permissions for build dir")
            call(cmd)

            cmd = ["docker", "run",
                   "-v", "{}:/debbuild/".format(tmp_dir),
                   "gin-deb", "dpkg-deb", "--build",
                   "/debbuild/{}".format(pkgname)]
            # call(["tree", "-L", "5", tmp_dir])
            print("Building deb package")
            ret = call(cmd)
            # cmd = ["docker", "run",
            #        "-v", "{}:/debbuild/".format(tmp_dir),
            #        "gin-deb", "dpkg", "--contents",
            #        "/debbuild/{}.deb".format(pkgname)]
            # ret = call(cmd)
            if ret > 0:
                print("Deb build failed", file=sys.stderr)
                continue

            # revert ownership to allow deletion of tmp dir
            uid = os.getuid()
            cmd = ["docker", "run", "-v", "{}:/debbuild/".format(tmp_dir),
                   "gin-deb", "/bin/bash", "-c",
                   "chown {}:{} -R /debbuild".format(uid, uid)]
            ret = call(cmd)
            if ret > 0:
                print("Error occured while reverting ownership to user",
                      file=sys.stderr)

            debfilename = "{}.deb".format(pkgname)
            debfilepath = os.path.join(tmp_dir, debfilename)
            debfiledest = os.path.join(pkgdir, debfilename)
            if os.path.exists(debfiledest):
                os.remove(debfiledest)
            shutil.copy(debfilepath, debfiledest)
            debs.append(debfiledest)
            print("DONE")
    return debs


def rpmify(binfiles, annexsa_archive):
    return []


def winbundle(binfiles, git_pkg, annex_pkg):
    """
    For each Windows binary make a zip and include git and git annex portable
    """
    winarchives = []
    for bf in binfiles:
        with TemporaryDirectory(suffix="gin-windows") as tmp_dir:
            pkgname = "gin-cli_{}".format(version["version"])
            pkgroot = os.path.join(tmp_dir, "gin")
            bindir = os.path.join(pkgroot, "bin")
            os.makedirs(bindir)

            shutil.copy(bf, bindir)
            shutil.copy("README.md", pkgroot)
            shutil.copy("gin.bat", pkgroot)

            gitdir = os.path.join(pkgroot, "git")
            os.makedirs(gitdir)

            # extract git portable and annex into git dir
            cmd = ["7z", "x", "-o{}".format(gitdir), git_pkg]
            print("Running {}".format(" ".join(cmd)))
            ret = call(cmd, stdout=DEVNULL)
            if ret > 0:
                print("Failed to extract git archive [{}]".format(git_pkg),
                      file=sys.stderr)
                continue

            cmd = ["7z", "x", "-o{}".format(gitdir), annex_pkg]
            print("Running {}".format(" ".join(cmd)))
            ret = call(cmd, stdout=DEVNULL)
            if ret > 0:
                print("Failed to extract git archive [{}]".format(annex_pkg),
                      file=sys.stderr)
                continue
            d, f = os.path.split(bf)
            _, osarch = os.path.split(d)

            arc = "gin-cli_{}-{}.zip".format(version["version"], osarch)
            arc = os.path.join(pkgdir, arc)
            print("Creating Windows zip file")
            # need to change paths before making zip file
            if os.path.exists(arc):
                os.remove(arc)
            arc_abs = os.path.abspath(arc)
            oldwd = os.getcwd()
            os.chdir(pkgroot)
            cmd = ["zip", "-r", arc_abs, "."]
            print("Running {} (from {})".format(" ".join(cmd), pkgroot))
            ret = call(cmd, stdout=DEVNULL)
            os.chdir(oldwd)
            if ret > 0:
                print("Failed to create archive [{}]".format(arc),
                      file=sys.stderr)
                continue
            winarchives.append(arc)
            print("DONE")
    return winarchives


def main():
    os.makedirs(os.path.join(destdir, "downloads"), exist_ok=True)
    os.makedirs(pkgdir, exist_ok=True)

    incbuild = "--incbuild" in sys.argv
    binfiles = build(incbuild)
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

    win_pkgs = winbundle(win_bins, win_git_file, win_git_annex_file)

    def printlist(lst):
        print("".join("> " + l + "\n" for l in lst))

    print("------------------------------------------------")
    print("The following archives and packages were created")
    print("------------------------------------------------")
    print("Linux tarballs:")
    printlist(linux_pkgs)

    print("Debian packages:")
    printlist(deb_pkgs)

    print("RPM packages:")
    printlist(rpm_pkgs)

    print("Windows packages:")
    printlist(win_pkgs)


if __name__ == "__main__":
    main()
