FROM ubuntu:18.04
ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update && apt-get install curl gnupg -y

RUN curl https://nodejs.org/dist/v12.1.0/node-v12.1.0-linux-x64.tar.gz | tar xz -C /usr/local
ENV PATH=/usr/local/node-v12.1.0-linux-x64/bin:$PATH

RUN curl https://dl.google.com/go/go1.12.4.linux-amd64.tar.gz | tar xz -C  /usr/local
ENV PATH=/usr/local/go/bin:$PATH
ENV PATH=/root/go/bin:$PATH
ENV GO111MODULE=on

RUN curl -sS https://dl.yarnpkg.com/debian/pubkey.gpg | apt-key add - \
    && echo "deb https://dl.yarnpkg.com/debian/ stable main" > /etc/apt/sources.list.d/yarn.list \
    && apt update -y \
    && apt install yarn -y

RUN apt-get install build-essential git postgresql postgresql-contrib -y
RUN    echo local all all     trust > /etc/postgresql/10/main/pg_hba.conf \
    && echo host  all all all trust >> /etc/postgresql/10/main/pg_hba.conf \
    && service postgresql start \
    && su postgres -c 'psql -c "create role root with superuser password null login"' \
    && su postgres -c 'psql -c "create database root"'

WORKDIR /app
COPY go.mod .
COPY go.sum .
RUN go mod download

ADD . /app

RUN go install golang.org/x/tools/cmd/goimports \
    && go install ./cmd/testbot
CMD service postgresql start && /root/go/bin/testbot worker
