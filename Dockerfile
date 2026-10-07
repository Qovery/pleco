FROM public.ecr.aws/r3m4q3r9/pub-mirror-go:1.21.6-bullseye AS build

ADD . /pleco
WORKDIR /pleco
RUN go mod download && go build -mod=readonly -o /pleco.bin main.go

FROM public.ecr.aws/r3m4q3r9/pub-mirror-debian:bookworm-slim AS run

RUN apt-get update && apt-get install -y ca-certificates curl gnupg python3 && apt-get clean
# gcloud CLI to connect to GCP clusters
RUN echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" > /etc/apt/sources.list.d/google-cloud-sdk.list \
    && curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg -o /tmp/google-cloud-apt-key.gpg \
    && gpg --dearmor -o /usr/share/keyrings/cloud.google.gpg /tmp/google-cloud-apt-key.gpg \
    && apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y google-cloud-cli google-cloud-cli-gke-gcloud-auth-plugin \
    && rm -rf /var/lib/apt/lists/* /tmp/google-cloud-apt-key.gpg
COPY --from=build /pleco.bin /usr/bin/pleco
CMD ["pleco", "start"]
