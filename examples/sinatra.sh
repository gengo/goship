#!/bin/bash
export RAILS_ENV=production

export PROJECT_PATH='/home/deploy/mysinatraapp'

mkdir -p $PROJECT_PATH/shared/log
mkdir -p $PROJECT_PATH/shared/tmp
mkdir -p $PROJECT_PATH/shared/bundle

# assuming we have git repo in repo/ dir
cd $PROJECT_PATH/repo
git pull

# remove symlinks
rm ${PROJECT_PATH}/repo/log ${PROJECT_PATH}/repo/tmp

# recreate symlinks
ln -s ${PROJECT_PATH}/shared/log ${PROJECT_PATH}/repo/log
ln -s ${PROJECT_PATH}/shared/tmp ${PROJECT_PATH}/repo/tmp

# bundle the project
bundle install --deployment --path ${PROJECT_PATH}/shared/bundle

sv restart /home/deploy/service/mysinatraapp_puma
