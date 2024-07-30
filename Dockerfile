FROM public.ecr.aws/r3m4q3r9/pub-mirror-go:1.21.6-bullseye as build

ADD . /pleco
WORKDIR /pleco
RUN go get && go build -o /pleco.bin main.go

FROM public.ecr.aws/r3m4q3r9/pub-mirror-debian:bullseye-slim as run

RUN apt-get update && apt-get install -y ca-certificates curl gnupg python3 && apt-get clean
# gcloud CLI to connect to GCP clusters
RUN echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" | tee -a /etc/apt/sources.list.d/google-cloud-sdk.list && curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | gpg --dearmor -o /usr/share/keyrings/cloud.google.gpg && apt-get update -y && apt-get install google-cloud-sdk google-cloud-sdk-gke-gcloud-auth-plugin -y
COPY --from=build /pleco.bin /usr/bin/pleco
CMD ["pleco", "start"]
