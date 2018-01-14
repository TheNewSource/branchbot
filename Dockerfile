FROM scratch
MAINTAINER Danielle Tomlinson <dani@builds.terrible.systems>

ADD resources/ca-certificates.crt /etc/ssl/certs/
COPY ./build/release/branchbot-linux-amd64 /branchbot

ENTRYPOINT ["/branchbot"]
CMD ["-help"]
