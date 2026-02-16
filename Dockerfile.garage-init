FROM dxflrs/garage:v2.2.0 AS garage
FROM alpine:3.21
COPY --from=garage /garage /usr/local/bin/garage
