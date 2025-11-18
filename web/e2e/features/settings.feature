Feature: Settings Page
  As a user
  I want to manage application settings
  So that I can customize my experience

  Scenario: Display settings page
    When I navigate to the settings page
    Then I should see the "Settings" heading
    And the settings page should be displayed

  Scenario: Theme selection is available
    When I'm on the settings page
    Then I should see a theme selector
    And I should have the following theme options available:
      | theme |
      | Light |
      | Dark  |

  Scenario: Display current theme
    When I'm on the settings page
    And the current theme is "Dark"
    Then the "Dark" theme option should be selected

  Scenario: Change theme
    Given I'm on the settings page
    And the current theme is "Light"
    When I select the "Dark" theme
    Then the application theme should change to "Dark"
    And the theme preference should be saved

  Scenario: Theme changes apply immediately
    When I'm on the settings page
    And I select the "Dark" theme
    Then the background color should change immediately
    And all text colors should update to dark mode

  Scenario: Persist theme preference
    Given I have selected "Dark" theme in settings
    When I reload the application
    Then the "Dark" theme should still be active
