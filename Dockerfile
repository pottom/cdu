# Built by GoReleaser, not with `docker build .`: the pre-compiled linux binary for
# the target arch is copied into the build context, so there is no build stage here.
# scratch keeps the image to just the static, CGO-free binary — nothing else to run,
# nothing else to patch.
FROM scratch
COPY cdu /usr/bin/cdu
ENTRYPOINT ["/usr/bin/cdu"]
