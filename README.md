scrapper
========

Purpuse of this script is to show how GO language can be easily used as scripting language to automate parsing of data from html.

All the heavy lifting is done by h5 library 
http://code.google.com/p/go-html-transform/
Great little library, only thing I was missing was nested selectors, like jQuery has.

Script has support for custom base url, custom output file, parsing multiple products, or single product from url, also it can cache files on disk, to be faster if we have to rescan

TODO:

Refractor parsing into seperate library.  
Enable complex parsing of some cases where data is encoded into javascript.   
Implement parsing of difference between files on disk and online.   
