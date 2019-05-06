FROM ubuntu:18.04
ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update && apt-get install curl -y

RUN curl https://nodejs.org/dist/v12.1.0/node-v12.1.0-linux-x64.tar.gz | tar xz -C /usr/local
ENV PATH=/usr/local/node-v12.1.0-linux-x64/bin:$PATH

RUN curl https://dl.google.com/go/go1.12.4.linux-amd64.tar.gz | tar xz -C  /usr/local
ENV PATH=/usr/local/go/bin:$PATH

RUN apt-get install build-essential git postgresql postgresql-contrib -y
RUN echo local all all trust > /etc/postgresql/10/main/pg_hba.conf
RUN service postgresql start \
    && su postgres -c 'psql -c "create role root with superuser password null login"' \
    && su postgres -c 'psql -c "create database root"'

WORKDIR /app
ADD . /app
RUN go install ./cmd/testbot
CMD service postgresql start && /root/go/bin/testbot worker
