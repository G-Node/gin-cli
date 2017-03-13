# -*- coding: utf-8 -*-
import pickle
import sys
import os
import json
import re
from subprocess import check_output, call
try:
    import requests
    HAVEREQUESTS = True
except ImportError:
    print("Requests module not installed. Will not download packages.")
    requests = None
    HAVEREQUESTS = False

etagfile = "downloads/etags"
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
    fname = os.path.join("downloads", fname)
    print("--> Downloading {} → {}".format(url, fname))
    req = requests.get(url, stream=True)
    size = req.headers.get("content-length")
    et = req.headers.get("etag")
    oldet = etags.get(url)
    if et == oldet and os.path.exists(fname):
        fod_size = os.path.getsize(fname)
        if int(fod_size) == int(size):
            print("File already downloaded. Skipping.")
            return fname
    etags[url] = et
    prog = 0
    with open(fname, "wb") as fd:
        for chunk in req.iter_content(chunk_size=256):
            fd.write(chunk)
            prog += len(chunk)
            print("\r{}/{}".format(prog, size), end="", flush=True)
        print("\nDone!")
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
    plat = sys.platform
    if plat.startswith("win"):
        gin_exe = "gin.exe"
        linecont = "=>"
    else:
        gin_exe = "./gin"
        linecont = "↪"
    print("--> Building binary for [{}]".format(plat))
    verfile = "version"
    with open(verfile) as fd:
        verinfo = fd.read()

    verdict = {}
    verdict["version"] = re.search(r"version=([v0-9\.]+)", verinfo).group(1)
    verdict["build"] = re.search(r"build=([0-9]+)", verinfo).group(1)
    cmd = ["git", "rev-parse", "HEAD"]
    verdict["commit"] = check_output(cmd).strip().decode()
    print(("Version: {version} "
           "Build: {build} "
           "Commit: {commit}").format(**verdict))
    ldflags = ("-X main.version={version} "
               "-X main.build={build} "
               "-X main.commit={commit}").format(**verdict)
    cmd = ["go", "build", "-ldflags", ldflags, "-o", gin_exe]
    ret = call(cmd)
    if ret > 0:
        die("Build failed")

    print("--> Build succeeded")
    cmd = [gin_exe, "--version"]
    verstring = check_output(cmd).strip().decode()
    print("{}\n{} {}".format(" ".join(cmd), linecont, verstring))

    if incbuild:
        print("--> Updating version file")
        verdict["build"] = "{:05d}".format(int(verdict["build"]) + 1)
        newinfo = "version={version}\nbuild={build}\n".format(**verdict)
        print(newinfo)
        with open(verfile, "w") as fd:
            fd.write(newinfo)


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


def main():
    try:
        os.mkdir("downloads")
    except FileExistsError:
        pass
    incbuild = "--incbuild" in sys.argv
    dl = "--no-dl" not in sys.argv and HAVEREQUESTS
    linux_file = build(incbuild)
    if dl:
        load_etags()
        annexsa_file = download_annex_sa()
        win_url = get_appveyor_artifact_url()
        win_file = download(win_url, "gin.exe")
        win_git_file = get_git_for_windows()
        win_git_annex_file = get_git_annex_for_windows()
        save_etags()

        print("Ready to package")


if __name__ == "__main__":
    main()
