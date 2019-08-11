FROM python:3.7.3-slim

ENV FRONTEND_ORIGIN "http://localhost:8080/"
# install gcc
RUN apt-get update && \
    apt-get -y install gcc && \
    apt-get clean

# set working directory
WORKDIR /usr/src/app
# add and install requirements
COPY ./requirements.txt /usr/src/app/requirements.txt
RUN pip install -r requirements.txt

# add entrypoint.sh
COPY ./entrypoint.sh /usr/src/app/entrypoint.sh
RUN chmod +x /usr/src/app/entrypoint.sh

# add app
COPY . /usr/src/app

# run server
CMD ["/usr/src/app/entrypoint.sh"]