FROM shibuya:base

ARG binary_name=shibuya

ADD ./build/${binary_name} /usr/local/bin/${binary_name}
ENV GOOGLE_APPLICATION_CREDENTIALS /auth/shibuya-gcp.json

ARG env=local
ENV env ${env}

COPY ./ui/ /
ENV binary=${binary_name}
ENTRYPOINT ["/bin/sh", "-c", "exec ${binary}"]

USER ${USER}
