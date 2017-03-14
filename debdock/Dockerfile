FROM debian:jessie

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get --yes update
RUN apt-get --yes upgrade
RUN apt-get --yes install build-essential

RUN mkdir /debbuild

# The following makes the resulting deb file ownership match the running user
ARG userid
RUN useradd -u ${userid} builder
RUN chown builder:builder -R /debbuild
