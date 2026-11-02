FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends openssh-server less file iproute2 \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --create-home --shell /bin/bash student \
    && printf 'student:learn123\n' | chpasswd \
    && mkdir -p /run/sshd \
    && rm -f /etc/ssh/ssh_host_*

COPY sshd_config /etc/ssh/sshd_config
COPY entrypoint.sh /usr/local/bin/lab-entrypoint
COPY --chown=student:student student-files/ /home/student/lab/
RUN chmod 0755 /usr/local/bin/lab-entrypoint \
    && find /home/student/lab -type d -exec chmod 0755 {} + \
    && find /home/student/lab -type f -exec chmod 0644 {} +

EXPOSE 22
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD ss -ltnH 'sport = :22' | grep -q .
ENTRYPOINT ["/usr/local/bin/lab-entrypoint"]
