FROM golang
RUN curl -L https://cdn.porter.sh/latest/install-linux.sh | bash && \
    curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
    install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
ENV PATH "$PATH:/root/.porter"
RUN     mkdir -p /root/.porter/plugins/kubernetes  &&\
        mkdir -p /root/.porter/credentials
COPY / /app
RUN install /app/bin/plugins/kubernetes/kubernetes /root/.porter/plugins/kubernetes/kubernetes
WORKDIR /app
COPY ./tests/integration/scripts/config-*.toml /root/.porter/
COPY ./tests/testdata/kubernetes-plugin-test-*.json /root/.porter/credentials/
ENTRYPOINT ["tail","-f","/dev/null"]