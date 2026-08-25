FROM registry.access.redhat.com/ubi8/ubi-minimal:latest
LABEL org.opencontainers.image.source="https://github.com/paketo-buildpacks/ubi8-base-stack"
USER root
RUN mkdir -p /etc/buildpacks
COPY ./images.json /etc/buildpacks/images.json
RUN chmod 644 /etc/buildpacks/images.json
