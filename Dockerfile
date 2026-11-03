FROM public.ecr.aws/r3m4q3r9/pub-mirror-go:1.27.2 AS build

ADD . /pleco
WORKDIR /pleco
# Static binary: the Go image is Debian 13 (glibc 2.41) while the runtime image is Debian 12 (glibc 2.36)
RUN go mod download && CGO_ENABLED=0 go build -mod=readonly -o /pleco.bin main.go

# Fetch the gcloud apt signing key here so curl and gnupg stay out of the runtime image
FROM public.ecr.aws/r3m4q3r9/pub-mirror-debian:bookworm-slim AS gcloud-key
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl gnupg \
    && curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg | gpg --dearmor -o /cloud.google.gpg

FROM public.ecr.aws/r3m4q3r9/pub-mirror-debian:bookworm-slim AS run

# upgrade pulls Debian security fixes not yet in the mirrored base image
RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get upgrade -y && apt-get install -y ca-certificates python3 && apt-get clean
# gcloud CLI to connect to GCP clusters
COPY --from=gcloud-key --chmod=644 /cloud.google.gpg /usr/share/keyrings/cloud.google.gpg
RUN echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" > /etc/apt/sources.list.d/google-cloud-sdk.list \
    && apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y google-cloud-cli google-cloud-cli-gke-gcloud-auth-plugin \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /pleco.bin /usr/bin/pleco
CMD ["pleco", "start"]
