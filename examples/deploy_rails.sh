#!/bin/bash
export RAILS_ENV=production

export PROJECT_PATH='/home/deploy/myapp'

mkdir -p $PROJECT_PATH/shared/log
mkdir -p $PROJECT_PATH/shared/tmp
mkdir -p $PROJECT_PATH/shared/bundle

# assuming we have git repo in repo/ dir
cd $PROJECT_PATH/repo
git pull

# send stop signal to sidekiq
sv 2 /home/deploy/service/myapp_sidekiq

# remove symlinks
rm -rf ${PROJECT_PATH}/repo/log
rm ${PROJECT_PATH}/repo/tmp
rm ${PROJECT_PATH}/repo/config/database.yml
rm ${PROJECT_PATH}/repo/public/system

# recreate symlinks
ln -s ${PROJECT_PATH}/shared/log ${PROJECT_PATH}/repo/log
ln -s ${PROJECT_PATH}/shared/tmp ${PROJECT_PATH}/repo/tmp
ln -s ${PROJECT_PATH}/shared/config/database.yml ${PROJECT_PATH}/repo/config/database.yml
ln -s ${PROJECT_PATH}/shared/public/system ${PROJECT_PATH}/repo/public/system

# bundle
bundle install --deployment --path ${PROJECT_PATH}/shared/bundle
bundle exec rake db:migrate
bundle exec rake assets:precompile
# write cron tasks
bundle exec whenever --update-crontab

# restart app server
sv restart /home/deploy/service/myapp_puma

# restart sidekiq
sv restart /home/deploy/service/myapp_sidekiq
