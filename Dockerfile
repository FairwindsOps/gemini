FROM alpine:3.18

LABEL org.opencontainers.image.authors="FairwindsOps, Inc." \
      org.opencontainers.image.vendor="FairwindsOps, Inc." \
      org.opencontainers.image.title="gemini" \
      org.opencontainers.image.description="Automated backups of PersistentVolumeClaims in Kubernetes using VolumeSnapshots" \
      org.opencontainers.image.documentation="https://github.com/FairwindsOps/gemini" \
      org.opencontainers.image.source="https://github.com/FairwindsOps/gemini" \
      org.opencontainers.image.url="https://github.com/FairwindsOps/gemini" \
      org.opencontainers.image.licenses="Apache License 2.0"

WORKDIR /usr/local/bin
RUN apk -U upgrade
RUN apk --no-cache add ca-certificates

RUN addgroup -S gemini && adduser -u 1200 -S gemini -G gemini
USER 1200
COPY gemini .

WORKDIR /opt/app

CMD ["gemini"]
