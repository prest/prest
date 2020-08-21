# pREST docs
Public site

## Contributing

- Clone this repository.
```
git clone git@github.com:prest/prest.github.io.git
```
- Make sure you pull all branchs and change it to `gh-pages`.
```
git checkout -b gh-pages origin/gh-pages
```
- Update git submodules to getting Themes repository.
```
git submodule init
git submodule update
```
- Do your modifications into `content` folder following Hugo documentation and run the webserver on port 1313.
```
hugo server -D
```
- When you finish, commit it and send a Pull Request linking `gh-pages` branch.

## Deploy

On gh-pages branch, run command:
*Auto commit and push to master*
```
./deploy.sh
```
