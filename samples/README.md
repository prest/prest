# Samples to getting started with API calls

## Description

First api calls and test automation sample.

## Usage

Import on Postman and execute the following steps:

* Bearer Authentication
* List Databases

This is the manual process to see how things is going.

So, we have the automated way:

```
npm i --location=global newman
```

After the installation run the following command:

```
newman run samples/prest_first_look.postman_collection.json
```

That's it, you have a way to validate the project running locally, and to test on the environments you need to edit and go forward with your own version of this sample.