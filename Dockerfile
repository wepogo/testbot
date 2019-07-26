FROM ubuntu:18.04

# System Dependencies
ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update && apt-get install build-essential git curl gnupg vim -y

# Language Dependencies
RUN curl https://dl.google.com/go/go1.12.4.linux-amd64.tar.gz | tar xz -C  /usr/local \
    && curl https://nodejs.org/dist/v12.1.0/node-v12.1.0-linux-x64.tar.gz | tar xz -C /usr/local \
    && curl -sS https://dl.yarnpkg.com/debian/pubkey.gpg | apt-key add - \
    && echo "deb https://dl.yarnpkg.com/debian/ stable main" > /etc/apt/sources.list.d/yarn.list \
    && apt update -y \
    && apt install --no-install-recommends yarn -y

# Postgres Dependencies
RUN curl -sS https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add - \
    && echo "deb http://apt.postgresql.org/pub/repos/apt/ bionic-pgdg main" > /etc/apt/sources.list.d/pgdg.list \
    && apt-get update -y \
    && apt-get install postgresql-11 postgresql-contrib -y

# Postgres Configuration
RUN echo local all all trust > /etc/postgresql/11/main/pg_hba.conf \
    && echo host all all all trust >> /etc/postgresql/11/main/pg_hba.conf \
    && service postgresql start \
    && su postgres -c 'psql -c "create role testbot with superuser password null login"' \
    && su postgres -c 'psql -c "create database testbot"'

# Environment
# Do this after setup so we don't have to re-download for
# minor changes.

# Go Environment
ENV GO111MODULE=on
ENV GOPATH=/go
ENV PATH=/usr/local/go/bin:$PATH
ENV PATH=$GOPATH/bin:$PATH

# Node Environment
ENV PATH=/usr/local/node-v12.1.0-linux-x64/bin:$PATH

# Postgres Environment
ENV PGUSER=testbot
VOLUME ["/etc/postgresql", "/var/log/postgresql", "/var/lib/postgresql"]

# Build testbot worker
WORKDIR /testbot
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN go install ./cmd/testbot

CMD service postgresql start && testbot worker
