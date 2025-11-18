Feature: Vault View
  As a user
  I want to view and interact with vault content
  So that I can manage my notes and files

  Scenario: Load vault view
    Given I navigate to vault "default"
    Then the vault view should load
    And I should see the vault name "default" in the header

  Scenario: Display file tree sidebar
    Given I navigate to vault "default"
    Then I should see the file tree sidebar
    And the file tree should be displayed

  Scenario: SSE connection status indicator
    Given I navigate to vault "default"
    Then I should see a connection status indicator

  Scenario: Expand folder to show children
    Given I navigate to vault "default"
    When I view the vault view
    Then I should see the file tree sidebar

  Scenario: Display main content area
    Given I navigate to vault "default"
    Then I should see the main content area
