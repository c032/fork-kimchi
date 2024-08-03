FROM golang:1.22.5-alpine3.20 AS build

RUN apk update && apk add bmake scdoc

COPY . /usr/src/app
WORKDIR /usr/src/app
RUN mkdir /tmp/output && bmake all && bmake install DESTDIR=/tmp/output

FROM alpine:3.20.2

RUN addgroup -g '100000' 'kimchi' && adduser -D -G 'kimchi' -u '100000' 'kimchi'

COPY --from=build /tmp/output /

RUN apk update && apk upgrade && apk add dumb-init

RUN \
	mkdir -p /var/log/kimchi /srv/kimchi && \
	chown -R kimchi:kimchi /var/log/kimchi/ /srv/kimchi/

USER kimchi
WORKDIR /srv/kimchi

ENTRYPOINT ["/usr/bin/dumb-init", "--"]
CMD ["/usr/local/bin/kimchi"]
