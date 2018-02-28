#!/bin/bash
echo -e "\033[0;32mDeploying updates to GitHub...\033[0m"

echo "Removing existing files"
rm -rf public

echo "Generating site"
hugo

echo "Updating master branch"
msg="rebuilding site `date`, publishing to master"
git checkout master && ll --ignore public | xargs rm -rf && cp -rf public/* . && rm -rf public && git add . && git commit -m "$msg"

git push origin master

git checkout gh-pages
