FROM alpine:3.14.3

COPY check/check /opt/resource/
COPY in/in    /opt/resource/
COPY out/out   /opt/resource/
