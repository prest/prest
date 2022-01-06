ARG DOCKERTAG=rc
FROM registry.hub.docker.com/library/golang:${DOCKERTAG}

# Variables that will setup a non-root user.
ARG USERNAME=vscode
ARG USER_UID=1000
ARG USER_GID=$USER_UID

WORKDIR /workspace
ENV GOARCH amd64
ENV GOOS linux
ENV CGO_ENABLED 1
RUN go mod vendor && \
    go build -ldflags "-s -w" -o prestd cmd/prestd/main.go && \
	ln -s /workspace/prestd /bin/prestd && \
    apt-get update && \
	apt-get install --no-install-recommends -yq netcat && \
	apt-get autoremove -y && apt-get clean -y

ENTRYPOINT ["sh", "/app/entrypoint.sh"]
