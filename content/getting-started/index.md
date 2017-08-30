---
date: 2016-04-23T15:57:19+02:00
menu: "main"
title: Getting Started
weight: 10
---

## Installation

### Install Hugo

Hugo itself is just a single binary without dependencies on expensive runtimes like Ruby, Python or PHP and without dependencies on any databases. 

{{< admonition title="Note" type="note" >}}
Currently, this theme makes use of features that will be part of the Hugo v0.16 release. You have to [compile](https://github.com/spf13/hugo/#clone-the-hugo-project-contributor) the latest developement version yourself.
{{< /admonition >}}

<!--You just need to download the [latest version](https://github.com/spf13/hugo/releases). For more information read the official [installation guides](http://gohugo.io/overview/installing/). -->

Let's make sure Hugo is set up as expected. You should see a similar version number in your terminal:

```sh
hugo version
# Hugo Static Site Generator v0.15 BuildDate: 2016-01-03T12:47:47+01:00
```

### Install Alabaster

Next, assuming you have Hugo up and running the `hugo-alabaster` theme can be installed with `git`:

```sh
# create a new Hugo website
hugo new site my-awesome-docs

# move into the themes folder of your website
cd my-awesome-docs/themes/

# download the theme
git clone git@github.com:digitalcraftsman/hugo-alabaster-theme.git
```

## Setup

Next, take a look in the `exampleSite` folder at `themes/hugo-alabaster-theme/`. This directory contains an example config file and the content that you are currently reading. It serves as an example setup for your documentation. 

Copy at least the `config.toml` in the root directory of your website. Overwrite the existing config file if necessary. 

Hugo includes a development server, so you can view your changes as you go -
very handy. Spin it up with the following command:

```sh
hugo server
```

Now you can go to [localhost:1313](http://localhost:1313) and the Alabaster
theme should be visible. You can now start writing your documentation, or read
on and customize the theme through some options.

## Configuration

Before you are able to deploy your documentation you should take a few minute to adjust some information in the `config.toml`. Open the file in an editor:

```toml
baseurl = "http://replace-this-with-your-hugo-site.com/"
languageCode = "en-us"
title = "Alabaster"

disqusShortname = ""
googleAnalytics = ""

[params]
  name = "Alabaster"
  description = "A documentation theme for Hugo."
```

Add some metadata about your theme. The `name` and `description` will appear in the top left of the sidebar. Furthermore, you can enable tracking via Google Analytics by entering your tracking code.

To allow users direct feedback to your documentation you can also enable a comment section powered by Disqus. The comment section will appear on each page except the homepage. 


## Options

### Adding a custom favicon

Favicons are small small icons that are displayed in the tabs right next to the title of the current page. As with the logo above you need to save your favicon in `static/` and link it relative to this folder in the config file:

```toml
[params]
  favicon = "favicon.ico"
```

### Syntax highlighting

This theme uses the popular [Highlight.js](https://highlightjs.org/) library to colorize code examples. The default theme is called "Foundation" with a few small tweaks. You can link our own theme if you like. Again, store your stylesheet in the `static/` folder and set the relative path in the config file:

```toml
[params]
  # Syntax highlighting theme
  highlightjs  = "path/to/theme.css"
```

Alternatively, you can use Pygments to highlight code blocks. If `highlightjs` does not contain a path it defaults to the Pygments stylesheet. Read the [Hugo docs](https://gohugo.io/extras/highlighting#pygments) for more information.

If you used GitHub flavoured Markdown with code fences like below

````
```toml
[params]
  # Syntax highlighting theme
  highlightjs  = "path/to/theme.css"
```
````

you have to add the `pygmentsuseclasses = true` option to the config file.

### Small tweaks

This theme provides a simple way for making small adjustments, that is changing some margins, centering text, etc. Simply put the CSS and Javascript files that contain your adjustments in the `static/` directory (ideally in subdirectories of their own) and include them via the `custom_css` and `custom_js`
variables in your `config.toml`. Reference the files relative to `static/`:

```toml
[params]
  custom_css = [
    "foo.css",
    "bar.css"
  ]

  custom_js = ["buzz.js"]
```


## Sidebar

### Adding a logo

If your project has a logo, you can add it to the sidebar by defining the variable `logo`. Save your logo somewhere in the `static/` folder and reference the file relative to this location:

```toml
[params.sidebar]
  logo = "images/logo.png"
```

### Adding menu entries

Once you created your first content files you can link them manually in the sidebar on the left. A menu entry has the following schema:

```toml
[[menu.main]]
  name   = "Home"
  url    = "/"
  weight = 0
```

`name` is the title displayed in the menu and `url` the relative URL to the content. The `weight` attribute allows you to modify the order of the menu entries. A menu entry appears further down the more weight you add.

Instead of just linking a single file you can enhance the sidebar by creating a nested menu. This way you can list all pages of a section instead of linking them one by one (without nesting).

You need extend the frontmatter of each file content file in a section slightly. The snippet below registers this content file as 'child' of a menu entry that already exists.

```yaml
menu: main
weight: 0
```

`main` specifies to which menu the content file should be added. `main` is the only menu in this theme by default. `parent` let's you register this content file to an existing menu entry, in this case the `Home` link. Note that the parent in the frontmatter needs to match the name in `config.toml`.

### Buttons

Above the menu you can include multiple buttons, e.g. the number of stars on GitHub or the current build status of [TravisCI](https://travis-ci.org/).

{{< admonition title="Note" type="note" >}}
It is required that you add your username and the URL to the project you want to document. Otherwise the buttons will not be able to display any information.
{{< /admonition >}}

If your project is hosted on GitHub, add the repository link and your username to the
configuration. 

```toml
[params]
  github_user   = "digitalcraftsman"
  github_repo   = "hugo-alabaster-theme"
  github_banner = true
```

`github_banner` adds the black banner in the top right of each page and simply links to your project on GitHub.

Now you can define which buttons you want to show:

```toml
[params.sidebar]
  github_button  = true
  travis_button  = false
  codecov_button = false
  gratipay = ""
```

1. `github_button` displays the number of stars of your repository on GitHub
1. `travis_button` displays the current build status of your project
1. `codecov_button` display the code coverage of your tests thanks to [CodeCov](https://codecov.io/)
1. `gratipay` allows you to collect tips via [GratiPay](https://gratipay.com/). Enter your username after you signed up.


### Relations

Includes links to the previous and next page in a section those exist under the menu as "Related Topics". This makes the navigation for users easier. It can be disabled if each of your website section's contain only contain a single page. 

```toml
[params.sidebar]
   show_relations = true
```

## Footer

The footer is very simple. Your `copyright` notice can be formatted with Markdown. This can be handy if you want to format text slightly or you want to include a hyperlink. 

`show_powered_by` allows you hide the showcase of the used tools. Some promotion is always appreciated.

```toml
[params.footer]
  copyright = "[Digitalcraftsman](https://github.com/digitalcraftsman)"
  show_powered_by = true
```