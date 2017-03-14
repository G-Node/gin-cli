# -*- coding: utf-8 -*-
import pickle
import sys
import os
import shutil
import json
import re
from glob import glob
from subprocess import check_output, call
import requests
from requests.exceptions import ConnectionError
from tempfile import TemporaryDirectory

destdir = "dist"

etagfile = os.path.join(destdir, "etags")
etags = {}


def load_etags():
    try:
        with open(etagfile, "rb") as fd:
            etags.update(pickle.load(fd))
    except FileNotFoundError:
        print("--> No etags file found. Skipping load.")


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

    verdict = {}
    verdict["version"] = re.search(r"version=([v0-9\.]+)", verinfo).group(1)
    verdict["build"] = re.search(r"build=([0-9]+)", verinfo).group(1)
    cmd = ["git", "rev-parse", "HEAD"]
    verdict["commit"] = check_output(cmd).strip().decode()
    if incbuild:
        print("--> Updating version file")
        verdict["build"] = "{:05d}".format(int(verdict["build"]) + 1)
        newinfo = "version={version}\nbuild={build}\n".format(**verdict)
        print(newinfo)
        with open(verfile, "w") as fd:
            fd.write(newinfo)
    print(("Version: {version} "
           "Build: {build} "
           "Commit: {commit}").format(**verdict))
    ldflags = ("-X main.version={version} "
               "-X main.build={build} "
               "-X main.commit={commit}").format(**verdict)
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
    ginfiles = glob(os.path.join(destdir, "*", "gin*"))
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
    return download(win_git_url, "git-for-windows.exe")


def get_git_annex_for_windows():
    win_git_annex_url = ("https://downloads.kitenet.net/git-annex/windows/"
                         "current/git-annex-installer.exe")
    return download(win_git_annex_url)


def package_linux(binfiles, annexsa_archive):
    """
    For each Linux binary, make a standalone and a plain bin version.
    """
    # copy README into dist directory
    shutil.copy("README.md", destdir)
    for bf in binfiles:
        d, f = os.path.split(bf)
        # simple binary archive
        cmd = ["tar", "-czf", "{}.tar.gz".format(d), "-C", d, f, "README.md"]
        print("Running {}".format(" ".join(cmd)))
        ret = call(cmd)
        if ret > 0:
            print("Packaging failed", file=sys.stderr)

        # debian packaged with annex standalone
        with TemporaryDirectory(suffix="gin-linux") as tmp_dir:
            build_dir = os.path.join(tmp_dir, "gin-cli_0.3-1")
            opt_dir = os.path.join(build_dir, "opt")
            os.makedirs(opt_dir)
            cmd = ["tar", "-xzf", annexsa_archive, "-C", opt_dir]
            print("Running {}".format(" ".join(cmd)))
            ret = call(cmd)
            if ret > 0:
                die("Archive extraction failed")

            shutil.copytree("debdock/DEBIAN",
                            os.path.join(build_dir, "DEBIAN"))
            shutil.copy("README.md", opt_dir)

            uid = os.getegid()
            cmd = ["docker", "build", "--build-arg=userid={}".format(uid),
                   "-t", "gin-deb", "debdock/."]
            print("Preparing docker image for debian build")
            ret = call(cmd)
            if ret > 0:
                die("docker build failed")

            cmd = ["docker", "run", "--user={}".format(uid),
                   "-v", "{}:/debbuild/".format(tmp_dir),
                   "gin-deb", "dpkg-deb", "--build", "/debbuild/gin-cli_0.3-1"]
            print("Building deb package")
            ret = call(cmd)
            if ret > 0:
                die("Deb build failed")

            debfile = os.path.join(tmp_dir, "gin-cli_0.3-1.deb")
            shutil.move(debfile, destdir)
            print("DONE")


def main():
    os.makedirs(os.path.join(destdir, "downloads"), exist_ok=True)
    incbuild = "--incbuild" in sys.argv
    binfiles = build(incbuild)
    load_etags()
    annexsa_file = download_annex_sa()
    win_git_file = get_git_for_windows()
    win_git_annex_file = get_git_annex_for_windows()
    save_etags()

    print("Ready to package")

    linux_bins = [b for b in binfiles if "linux" in b]
    linux_pkg = package_linux(linux_bins, annexsa_file)


if __name__ == "__main__":
    main()
