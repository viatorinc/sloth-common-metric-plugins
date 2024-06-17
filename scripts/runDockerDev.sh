#!/bin/bash

docker run -it --env ostype=Linux -v $HOME/dev/code/viator-sloth-plugins:/src --rm local/viator-sloth-slo-plugins-dev "$@"
