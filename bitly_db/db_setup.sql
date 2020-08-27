create database bitly;
use bitly;
create table shortcodetolongurl ( code varchar(10) primary key, longurl mediumtext, access_count mediumint default 0, create_time datetime default current_timestamp );
