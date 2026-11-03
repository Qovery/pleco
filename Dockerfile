FROM public.ecr.aws/r3m4q3r9/pub-mirror-go:1.27.2 AS build

ADD . /pleco
WORKDIR /pleco
# Static binary so the runtime image needs no libc
RUN go mod download && CGO_ENABLED=0 go build -mod=readonly -o /pleco.bin main.go

# No OS packages in the runtime image: pleco only needs CA certificates to reach cloud APIs
FROM scratch AS run
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /pleco.bin /usr/bin/pleco
# The Helm chart runs "pleco" through PATH
ENV PATH=/usr/bin
CMD ["pleco", "start"]
