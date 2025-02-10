FROM locustio/locust:2.32.8

# TODO maybe we should move this to non-root mode?
RUN pip install locust-plugins hyper

USER root
ARG GROUP=shibuya
ARG USER=shibuya-agent
ENV AGENT_ROOT=/shibuya-agent
RUN groupadd -r $GROUP && useradd -r -g $GROUP -m -d /home/$USER -s /bin/bash $USER
RUN mkdir -p $AGENT_ROOT && chown -R $USER:$GROUP $AGENT_ROOT
RUN mkdir /test-data && chown -R $USER:$GROUP /test-data
RUN chown ${USER}: /home/locust

USER ${USER}

RUN mkdir $AGENT_ROOT/test-conf $AGENT_ROOT/test-result

ADD build/locust-agent /usr/local/bin/locust-agent

# need below line to stop locust running when the container starts
ENTRYPOINT []

CMD ["locust-agent"]

