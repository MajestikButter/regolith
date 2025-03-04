---
permalink: /docs/profiles
layout: single
classes: wide
title: Profiles
sidebar:
  nav: "sidebar"
---

A `profile` is a collection of filters, settings, and export information. By default, a Regolith project will be initialized with a single profile, called `dev`. You can add additional profiles, as you need them.

## Running Profiles

You can use `regolith run` to run the default profile (dev), or use `regolith run <filter name>` to run a specific profile

## Why Profiles?

Profiles are useful for creating different run-targets. 

For example, `dev` profile may contain development focused filters, which are not desired for a final build. You can create a `build` or `package` profile, potentially with a different export target to fill this need. 

You can now run `regolith run dev` normally, and then sometimes `regolith run build` when you need a new final build.

## Profile Customization

For the most part, any setting inside of the Regolith config can be overridden inside of a particular profile. 

For example, `dataPath` can be defined at the top level, but customized per-profile if desired, by placing the key again inside of the profile: This path will be used when running this filter.

You can learn more about the configuration options available in Regolith [here](/regolith/docs/configuration).
