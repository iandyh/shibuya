FROM shibuya:base

ARG binary_name=shibuya

ARG COORDINATOR_FILES=/coordinator/files

ADD ./build/${binary_name} /usr/local/bin/${binary_name}
RUN mkdir -p $COORDINATOR_FILES && chown -R $USER:$GROUP $COORDINATOR_FILES

ARG env=local
ENV env ${env}

ENV binary=${binary_name}
CMD ${binary}

USER ${USER}
