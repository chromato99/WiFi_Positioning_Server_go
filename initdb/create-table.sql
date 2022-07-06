USE WiFi_Pos;

CREATE TABLE wifi_data (
    id INTEGER auto_increment,
    position varchar(200) NOT NULL,
    wifi_data JSON NOT NULL,
    PRIMARY KEY (id)
);