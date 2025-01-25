FROM shibuya:base

ARG binary_name=shibuya

ADD ./build/${binary_name} /usr/local/bin/${binary_name}
RUN mkdir -p /coordinator/files

ARG env=local
ENV env ${env}

ENV binary=${binary_name}
CMD ${binary}
