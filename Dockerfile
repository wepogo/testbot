FROM ubuntu:18.04

# System Dependencies
ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update && apt-get install build-essential git curl gnupg -y

# Language Dependencies
RUN curl https://nodejs.org/dist/v12.1.0/node-v12.1.0-linux-x64.tar.gz | tar xz -C /usr/local \
    && curl https://dl.google.com/go/go1.12.4.linux-amd64.tar.gz | tar xz -C  /usr/local \
    && curl -sS https://dl.yarnpkg.com/debian/pubkey.gpg | apt-key add - \
    && echo "deb https://dl.yarnpkg.com/debian/ stable main" > /etc/apt/sources.list.d/yarn.list \
    && apt update -y \
    && apt install --no-install-recommends yarn -y

# Postgres Dependencies
RUN apt-get install postgresql postgresql-contrib -y

# Postgres Configuration
RUN echo local all all trust > /etc/postgresql/10/main/pg_hba.conf \
    && echo host all all all trust >> /etc/postgresql/10/main/pg_hba.conf \
    && service postgresql start \
    && su postgres -c 'psql -c "create role root with superuser password null login"' \
    && su postgres -c 'psql -c "create database root"'

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


# Build testbot worker
WORKDIR /root
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN go install golang.org/x/tools/cmd/goimports
RUN go install ./cmd/testbot

CMD service postgresql start && testbot worker
