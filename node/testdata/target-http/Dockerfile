FROM scratch
COPY server /server
EXPOSE 8080
HEALTHCHECK --interval=1s --timeout=2s --retries=3 CMD ["/server", "probe"]
ENTRYPOINT ["/server"]
