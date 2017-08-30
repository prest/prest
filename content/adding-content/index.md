---
date: 2016-04-23T20:26:34+02:00
menu: "main"
title: Adding content
weight: 20
---

## Hello world

Let's create our first content file for your documentation. Open a terminal and add the following command for each new file you want to add. Replace `<section-name>` with a general term that describes your document in detail.

```sh
hugo new <section-name>/filename.md
```

Visitors of your website will find the final document under `www.example.com/<section-name>/filename/`.

Since it's possible to have multiple content files in the same section I recommend to create at least one `index.md` file per section. This ensures that users will find an index page under `www.example.com/<section-name>`.

## Homepage

To add content to the homepage you need to add a small indicator to the frontmatter of the content file:

```toml
type: homepage
```

Otherwise the theme will not be able to find the corresponding content file.

## Table of contents

You maybe noticed that the menu on the left contains a small table of contents of the current page. All `<h2>` tags (`## Headline` in Markdown) will be added automatically.

## Admonitions

Admonition is a handy feature that adds block-styled side content to your documentation, either notes or warnings. It can be enabled by using the corresponding [shortcodes](http://gohugo.io/extras/shortcodes/) inside your content:

```md
{{</* admonition title="Note" type="note" */>}}
Something good to know.
{{</* /admonition */>}}
```

will be rendered as

{{< admonition title="Note" type="note" >}}
Lorem ipsum dolor.
{{< /admonition >}}

The `type` parameter can optionally be used to either display a `note` or `warning`. The last type is the default one. 

```md
{{</* admonition title="Caution" */>}}
Don't try this at home!
{{</* /admonition */>}}
```

becomes

{{< admonition title="Caution" >}}
Don't try this at home!
{{< /admonition >}}
