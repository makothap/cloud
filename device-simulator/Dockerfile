FROM alpine:3.12 AS build

RUN apk add --no-cache curl git build-base gcc linux-headers patch
RUN git clone --recursive -b plgd.dev https://github.com/iotivity/iotivity-lite.git
COPY ./src/ /iotivity-lite/
RUN make -C /iotivity-lite/port/linux CLOUD=1 SECURE=1 DEBUG=1 MNT=1 cloud_server

FROM alpine:3.12 AS service
COPY --from=build /iotivity-lite/port/linux/cloud_server /usr/local/bin/cloud_server
ENTRYPOINT ["/usr/local/bin/cloud_server"]
