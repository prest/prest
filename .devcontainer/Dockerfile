ARG DOCKERTAG=1
FROM registry.hub.docker.com/library/golang:${DOCKERTAG}
COPY . /opt/prestd
COPY bashrc /root/.bashrc
ENV GOARCH amd64
ENV GOOS linux
ENV CGO_ENABLED 1
RUN apt-get update && \
	apt-get install --no-install-recommends -yq netcat postgresql-client && \
	apt-get autoremove -y && apt-get clean -y && \
	go install github.com/uudashr/gopkgs/v2/cmd/gopkgs@latest && \
	go install github.com/ramya-rao-a/go-outline@latest && \
	go install github.com/cweill/gotests/gotests@latest && \
	go install github.com/fatih/gomodifytags@latest && \
	go install github.com/josharian/impl@latest && \
	go install github.com/haya14busa/goplay/cmd/goplay@latest && \
	go install github.com/go-delve/delve/cmd/dlv@latest && \
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest && \
	go install golang.org/x/tools/gopls@latest
