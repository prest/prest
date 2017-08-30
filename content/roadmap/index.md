---
date: 2016-04-23T20:08:11+01:00
title: Roadmap
menu: main
weight: 30
---

Quo vadis? The port of the original [Alabaster theme](https://github.com/bitprophet/alabaster) has replicated nearly all of its features. A few are still missing, but I've good news: the Hugo community is actively working on this issues. Maybe with the next release of Hugo we can abandon this list. Stay tuned.

## Planned features

### Localization

Currently, it is possible to collect all strings in a single place for easy customization. However, this only enables you to define all strings in a single language. This approach is quite limiting in terms of localization support. Therefore, I decided to wait for a native integration. This way we can avoid a second setup of all strings in your website.

Keep an eye on [#1734](https://github.com/spf13/hugo/issues/1734).

### Search

Beside third-party services, some hacky workarounds and Grunt-/Gulp-based scripts that only require unnecessary dependencies, future versions of Hugo will support the generation of a content index as a core feature.

Keep an eye on [#1853](https://github.com/spf13/hugo/pull/1853).

### Styling options

The original theme allowed users to customize the layout directly from the [configuration](https://github.com/bitprophet/alabaster#style-colors). There is a similar idea that tries to template CSS stylesheets - this means every stylesheet would be treat as a template that can access the values from your config file.

Keep an eye on [#1431](https://github.com/spf13/hugo/pull/1431).

## Contributing

Did you found an bug or you would like to suggest a new feature? I'm open for feedback. Please open a new [issue](https://github.com/digitalcraftsman/hugo-alabaster-theme/issues) and let me know what you think.

You're also welcome to contribute with [pull requests](https://github.com/digitalcraftsman/hugo-alabaster-theme/pulls).