FROM scratch

COPY ./build/release/branchbot-linux-amd64 /branchbot

ENTRYPOINT ["/branchbot"]
CMD ["-help"]
