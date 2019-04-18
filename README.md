# Offline Server

This server currently handles processing raw events, cleaning their fields and finally publishing them to an Elastic Search database.

### Shape of an event document

On offline, an event can be:
- A single, one-time event
- A multi-day event
- A recurring event

To account for all of these cases, an event document is created for each occurrence of the event. Each occurrence will have a unique `startDateTime` and `endDateTime` timestamp indicating the physical start/end times for this occurrence of the event.

An example:
"Free Beer" 2pm-4pm 4/12 - 4/16 would create:
- "Free Beer" 4/12 2pm-4pm
- "Free Beer" 4/13 2pm-4pm
- "Free Beer" 4/14 2pm-4pm
- "Free Beer" 4/15 2pm-4pm
- "Free Beer" 4/16 2pm-4pm

Each occurrence shares the same `eventId` so you can associate them together, but they have a unique `id` across them.

```js
{
          "createDate": <Unix Timestamp>,
          "modifiedDate": <Unix Timestamp>,
          "address": "123 StreetName St, City, ST 12345, USA",
          "category : "inspiration|etc", // Offline category
          "description": "This is the description of the event.",
          "endDateTime" : <Unix Timestamp>,
          "eventUrl" : "https://www.facebook.com/events/12345678", // link to event
          "eventId" : "category-event-title", // ID shared by all occurrences
          "id" : "category-event-title:<StartDateTime>:<EndDateTime>", // Unique ID for this occurrence
          "imageUrl" : "https://url-of-the.image.jpg",
          "location" : {
            "lat" : 12.345678,
            "lon" : -12.345678
          },
          "offlineUrl" : "https://www.get-offline.com/category/event-title",
          "price" : 0,
          "referralUrls" : [ // Any urls found in the event description
            "https://moor-urls.com"
          ],
          "startDateTime" : <Unix Timestamp>,
          "tags" : [ ], // Any tags associated with the event
          "teaser" : "Short teaser title of the event",
          "title" : "Title of the event"
        }
```

## Local Setup

This setup requires go version `1.11+`

1. Clone and enter the repository
1. run `go get -u`
1. run `go build`
1. run `ELASTIC_SEARCH_URL=<URL> ./offline-server`

The server will host itself at `http://localhost:3000`;

## Running with Docker

To run with docker, you first have to create the docker image by executing:

```
run `docker build -t offline-server`
```

Once the image is created, you can create/run the container by executing:

```
docker run \
--env ELASTIC_SEARCH_URL=<URL> \
-d -p 3000:3000 \
--name offline-server \
offline-server
```

The docker run port mapping will host the server at `http://localhost:3000`;
