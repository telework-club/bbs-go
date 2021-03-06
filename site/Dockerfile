# builder stage
FROM node:14.5-buster AS builder

# Install build dependencies
RUN apt-get -qq update && \
    apt-get -qq install -y --no-install-recommends \
      build-essential \
      git \
      openssh-client \
      locales \
    && rm -rf /var/lib/apt/lists/*

# Set locale: https://daten-und-bass.io/blog/fixing-missing-locale-setting-in-ubuntu-docker-image/
RUN sed -i -e 's/# en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen && \
    dpkg-reconfigure --frontend noninteractive locales && \
    update-locale LANG=en_US.UTF-8
ENV LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8

# Update timezone
ENV TZ=Asia/Shanghai

WORKDIR /app

# Install dependencies
COPY package.json* ./

# RUN npm install -g yarn
RUN yarn install

FROM builder AS application

# Copy application code
COPY . .

# Build project
RUN yarn build

CMD ["yarn", "start"]
