# Goship Plugins

You can now add plugins to achieve greater developer happiness with Goship.

Currently, Goship allows for plugins to extend the columns of every project on the home page.
This is useful as we may wish to show additional details about each projects. For instance, we may wish to add an additional column to show the current health of the repo (e.g., latest Travis test results)

An example of a Travis status badge plugin can be found in the [travis folder](travis).

## Implementing a Goship Plugin

> TODO

## Adding Plugins to Goship

To ensure that plugins are implemented onto the Goship application, simply import the plugin in the main [`plugins/plugins.go` file](plugins/plugins.go) as shown.

Example:

```go
// in plugins/plugins.go
package plugins

// Import plugin packages here
import _ "github.com/gengo/goship/plugins/travis"

```

With this, when Goship is run, we should see the `RenderDetail()` and `RenderHeader()` method of our HelloWorld plugin displaying on the home page!

![helloworld plugin example](http://i.imgur.com/0r0yZGI.png)
