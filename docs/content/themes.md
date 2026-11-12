# Themes

Warden Web UI includes **16 embedded themes**, including Light, Dark, Catppuccin variants, Nord, Dracula, Gruvbox, Solarized, One Dark, Tokyo Night, and Rosé Pine variants.

## Select a theme

After signing in, open **Preferences → Appearance**, select a theme, and save it. The preference is linked to your local account and follows you across browsers and authentication modes. Public pages and new accounts use the **Light** default.

## Add a custom palette

Copy any TOML file from the [themes directory](https://github.com/gi8lino/warden-webui/tree/main/themes), edit its semantic color values, and mount it on the Web UI host or container. Then set:

```sh
WARDEN_WEBUI__THEME_DIRECTORY=/config/themes
```

The filename (without `.toml`) becomes the theme's display name. A custom file with the same name as an embedded theme overrides the built-in palette. The directory is read on startup, so restart Web UI after changing its contents.

For Kubernetes, mount the theme directory read-only from a ConfigMap or another trusted source. Do not include secrets in theme files.
