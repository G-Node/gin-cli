FROM debian:buster-slim

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get --yes update
RUN apt-get --yes upgrade
RUN apt-get --yes install build-essential lintian

RUN mkdir /debbuild
VOLUME ["/debbuild"]

ENTRYPOINT ["/debbuild/makedeb"]
