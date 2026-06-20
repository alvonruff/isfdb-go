
This is a desktop, read-only version of the ISFDB. It is written in go, and requires no external database or web server
installation. The goal is to provide ISFDB content to a broad audience, without relying on a public website, and has a
far easier installation process than trying to install the full Python source with Apache and MySQL on a home Linux system.
Note: The initial checkin has only been run on MacOS. Other operating systems will be tested in the next update.

# Installation

## Prerequisites

- [Go](https://go.dev/dl/) 1.21 or later
- A C compiler (required by the SQLite driver): on macOS this is provided by Xcode Command Line Tools (`xcode-select --install`); on Linux, `gcc` via your package manager.

## Steps

1. **Clone the repository**
   ```
   git clone https://github.com/alvonruff/isfdb-go.git
   cd isfdb-go
   ```

2. **Run the server**
   ```
   go run ./cmd/server
   ```
   On first run, no database is present. The server will start in setup mode and print the local address to the terminal.

3. **Download the database**

   Open a browser and navigate to `http://localhost:8080`. You will be taken to the setup page, which automatically begins downloading and importing the ISFDB SQLite database from Google Drive. The download is several hundred megabytes; import may take a minute or two depending on your hardware.

4. **Restart the server**

   Once the setup page reports that the import is complete, stop the server (`Ctrl-C`) and run it again:
   ```
   go run ./cmd/server
   ```
   The server is now ready. Point your browser to `http://localhost:8080` to browse the ISFDB.

## Keeping the database current

Navigate to `http://localhost:8080/update.cgi` at any time to check for a newer database snapshot and install it. The server must be restarted after an update completes.


# Motivation

Beginning in Feb 2026, the ISFDB began to suffer from waves of AI crawler attacks that left the site unusable. These
attacks are virtually indistinguisable from a DDOS attack: they consist of thousands of servers, with unrelated IP addresses,
coming from multiple countries, and coordinated from a central location.  This situation now appears to be a permanent feature 
of the Internet, as we are engaged in an arms race where every solution implemented by a web site is eventually countered by
the web crawlers. This situation puts the existence of small web sites that have large amounts of data in peril.

The ISFDB has a relatively turn-of-the-century web site architecture: all assets are hosted on a single server. The database, the
web server, the wiki, the editing and moderation tools are all operating on a single server. Any attack on one part of the
this ecosystem brings down the entire ecosystem. So some time has been spent on designing an architecture proposal that distributes assets 
across several servers: hiding the editing, moderation, and wiki assets behind a firewall, and creating a public, read-only server.
This would allow the bots to take down the public server (which would have a greatly reduced attack surface due to its read-only nature), 
without affecting the ability to continue bibliographic work on the private network.  However, given the increasing frequency of the 
web crawler attacks, this architecture would essentially be abandoning public access to the material. As such, an easily installable 
version of the ISFDB is desirable.

The difficulties and annoyances associated with the project to port the ISFDB from Python2 to Python3 highly influenced the language selection.
The degree of difficulty in installing and configuring MySQL, Apache, and Python, all contributed to this desire to create a desktop
version that is trivial to install. While the first version of this project remained on Python, it made the following radical changes:

* It deleted everything to do with editing. So /edit, /mod, /nightly, /rest, /scripts were all excised.
* All SQL tables/scripts dealing with editing were removed.
* All SQL tables/scripts dealing with the wiki were removed. Which removed user accounts.
* Since user accounts were removed, so were features that relied on user accounts: votes, tags, and verifications
* Removed the requirement for installing Apache by using Python's built-in web server
* Removed the requirement for installing MySQL and its connector, by using Python's built-in support for SQLite.

Once this version was operational, we then removed the last of the offenders: Python itself. Moving to a compiled language
brought with it a significant performance boost. 

# Development

This project is also an experiment on the current maturity of AI-assisted code development. For this project, we are currently using
Claude Code (Claude Sonnet 4.6). The development strategy is to treat the AI as a co-developer, working on small chunks of the size
I would personnaly work on, if I were programming alone. This makes the process more like a pair-programming approach, which turns out to be
difficult with humans, but easy with an AI.

# License

Background on MIT vs GPL. 

When Claude was queried as to which license it would prefer for the code it generated, it said: MIT.  When I told Claude that I was 
leaning towards an MIT-0 license, it responded with:

    "MIT-0 is a well-reasoned choice for this project specifically. A few thoughts on why it fits particularly well here:

    The attribution argument is weak in this context. The meaningful attribution in a bibliographic project belongs 
    to the data contributors — the thousands of people who entered and verified records in the ISFDB database. Requiring 
    attribution to the code authors while the data contributors get none would be philosophically backwards.

    It removes the one practical friction point of MIT. The standard MIT license is already minimal, but the attribution 
    clause creates a small compliance burden — someone building on this code has to carry a copyright notice. MIT-0 eliminates 
    that entirely. For a reference tool where widespread use and adaptation is the goal, that matters.

    It's consistent with the project's nature as infrastructure. The code is a window into the data. Windows shouldn't 
    assert ownership over what people see through them."

Well said.


Size:
   Full website: 199,872 LOC
   biblio only:   30,646 LOC
   Go version:    xx,xxx LOC
