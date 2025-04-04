FROM quay.io/operator-framework/opm AS opm
FROM alpine:3
COPY --from=opm /bin/opm /bin/opm
