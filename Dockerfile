FROM debian:buster-slim as build

RUN apt-get update && apt-get install -y curl && \
    curl -sLo pleco.tgz https://github.com/Qovery/pleco/releases/download/v0.1/pleco_0.1_linux_amd64.tar.gz &&\
    tar -xzf pleco.tgz


FROM debian:buster-slim as run

COPY --from=build /pleco /usr/bin/pleco
CMD ["pleco", "start"]