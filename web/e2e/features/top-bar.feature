Feature: Top Navigation Bar
  As a user
  I want to navigate the application using the top bar
  So that I can access different sections

  Scenario: Display application logo
    When I view the application
    Then I should see the "Obsidian Web" logo

  Scenario: Display progress indicator
    When I view the application
    Then I should see a progress indicator in the top bar
    And the progress indicator should be animated

  Scenario: Display settings icon
    When I view the application
    Then I should see a settings icon
    And the settings icon should be clickable

  Scenario: Navigate to settings page
    Given I'm on the home page
    When I click the settings icon
    Then I should be navigated to the settings page
    And the current route should be "settings"

  Scenario: Settings icon has correct styling
    When I view the application
    Then the settings icon should display as "⚙️"
    And the settings icon should be styled as clickable
