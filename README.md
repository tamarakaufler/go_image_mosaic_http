# go_image_mosaic_http

## SYNOPSIS

This is a GO implementation of a web application for uploading a jpeg photo and transforming it to a mosaic built from a collection of images, based on the averaged colour value of the particular tile.

The implementation concurrently processes the images, then, again concurrently, assembles the mosaic, before presenting both the uploaded photo and the mosaic on the page.   
