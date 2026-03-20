FROM alpine:3.21 AS certs
RUN apk add --no-cache ca-certificates \
    && adduser -D -u 10001 a9s

FROM scratch
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=certs /etc/passwd /etc/passwd
COPY a9s /usr/local/bin/a9s
USER a9s
ENTRYPOINT ["a9s"]
