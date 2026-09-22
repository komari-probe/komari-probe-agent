FROM alpine:3.21

WORKDIR /app

# Docker buildx 会在构建时自动填充这些变量
ARG TARGETOS
ARG TARGETARCH

COPY --chmod=755 sonar-agent-${TARGETOS}-${TARGETARCH} /app/sonar-agent

RUN ln -sf /app/sonar-agent /app/komari-agent && touch /.sonar-agent-container && touch /.komari-agent-container

ENTRYPOINT ["/app/sonar-agent"]
# 运行时请指定参数
# Please specify parameters at runtime.
# eg: docker run sonar-agent -e example.com -t token
CMD ["--help"]
