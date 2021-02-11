FROM golang
RUN curl https://cdn.porter.sh/latest/install-linux.sh | bash
ENV PATH "$PATH:/root/.porter"
RUN     mkdir -p /root/.porter/plugins/kubernetes  &&\
        mkdir -p /root/.porter/credentials
COPY / /app
RUN install /app/bin/plugins/kubernetes/kubernetes /root/.porter/plugins/kubernetes/kubernetes
WORKDIR /app
COPY ./tests/integration/scripts/config.toml /root/.porter
COPY ./tests/testdata/kubernetes-plugin-test.json /root/.porter/credentials/.
ENTRYPOINT ["tail","-f","/dev/null"]