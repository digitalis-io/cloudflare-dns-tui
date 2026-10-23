Feature: Cloudflare API client

  Scenario: List zones sorted by name
    Given the API returns zones in arbitrary order
    When I fetch zones
    Then they are returned sorted by name ascending

  Scenario: Reject empty API token
    Given no API token is set
    When I fetch zones
    Then I get an error containing "API token is required"

  Scenario: Fetch zones across multiple pages
    Given the API has 2 pages of zones
    When I fetch zones
    Then I receive zones from all pages

  Scenario: Surface API error messages
    Given the API returns an authentication error
    When I fetch zones
    Then I get an error containing "Invalid API token"

  Scenario: Surface non-200 HTTP status without error body
    Given the API returns HTTP 503 with no error detail
    When I fetch zones
    Then I get an error containing "HTTP 503"

  Scenario: Fetch DNS records with proxied and TTL fields
    Given the API returns two DNS records
    When I fetch records for zone "zone1"
    Then I receive 2 records
    And the first record has proxied=true and TTL=300
    And the second record has proxied=false and TTL=1

  Scenario: Concatenate multiple error messages with semicolon
    Given the API returns two error messages "err one" and "err two"
    When I fetch records for zone "zone1"
    Then I get an error containing "err one; err two"
